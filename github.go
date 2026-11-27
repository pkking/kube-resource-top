package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// ghPR is what the board shows for a task mapped to GitHub (ADR 0045).
// Zero PR means the API associates no PR — branch and workflow still show.
type ghPR struct {
	PR       int    `json:"pr,omitempty"`
	RunURL   string `json:"runUrl,omitempty"`
	JobURL   string `json:"jobUrl,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Workflow string `json:"workflow,omitempty"`
}

// ghClient keeps a runner name → PR cache, refreshed from the GitHub API
// on a 2-minute tick with a hard per-refresh call budget so the 5000/h
// authenticated quota is never at risk.
type ghClient struct {
	token    string
	hc       *http.Client
	mu       sync.RWMutex
	byRunner map[string]ghPR
}

func newGHClient(path string) (*ghClient, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	token := strings.TrimSpace(string(b))
	if token == "" {
		return nil, fmt.Errorf("empty token file")
	}
	return &ghClient{token: token, hc: &http.Client{Timeout: 20 * time.Second}, byRunner: map[string]ghPR{}}, nil
}

func (g *ghClient) get(url string, out interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := g.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("github %s: %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (g *ghClient) start(ctx context.Context, a *webApp) {
	// Startup retries: the first refreshes race the watcher sync, so keep
	// trying briefly until the cache holds something (or settle for the
	// steady 2-minute tick when nothing maps at all, e.g. at night).
	for i := 0; i < 5; i++ {
		select {
		case <-ctx.Done():
			return
		case <-time.After(20 * time.Second):
		}
		g.refresh(a)
		g.mu.RLock()
		n := len(g.byRunner)
		g.mu.RUnlock()
		if n > 0 {
			break
		}
	}
	tick := time.NewTicker(2 * time.Minute)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			g.refresh(a)
		}
	}
}

type ghPull struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
}

type ghRun struct {
	ID           int64    `json:"id"`
	Status       string   `json:"status"`
	HeadBranch   string   `json:"head_branch"`
	HeadSHA      string   `json:"head_sha"`
	HTMLURL      string   `json:"html_url"`
	Path         string   `json:"path"`
	PullRequests []ghPull `json:"pull_requests"`
}

type ghJob struct {
	Name       string `json:"name"`
	RunnerName string `json:"runner_name"`
	HTMLURL    string `json:"html_url"`
}

// refresh walks projects that currently hold running tasks: recent runs →
// in-progress jobs → runner names (ADR 0045). Any API failure skips that
// piece; the cache degrades to missing entries, never to guessed numbers.
func (g *ghClient) refresh(a *webApp) {
	const budget = 80 // calls per refresh, ≤ 2400/h
	projects := map[string]bool{}
	st := a.state()
	for _, res := range st.Resources {
		for _, t := range res.Tasks {
			if t.Run > 0 && strings.Contains(t.Project, "/") {
				projects[t.Project] = true
			}
		}
	}
	next := map[string]ghPR{}
	calls := 0
	for project := range projects {
		if calls >= budget {
			break
		}
		var runs struct {
			WorkflowRuns []ghRun `json:"workflow_runs"`
		}
		// Only in-progress runs carry runner names in their jobs; on a busy
		// repo they fall outside a plain recent-runs list, so filter by
		// status instead of paging through completed history.
		if err := g.get("https://api.github.com/repos/"+project+"/actions/runs?per_page=15&status=in_progress", &runs); err != nil {
			continue
		}
		calls++
		active := 0
		for _, run := range runs.WorkflowRuns {
			if calls >= budget || active >= 5 || run.Status != "in_progress" {
				continue
			}
			var jobs struct {
				Jobs []ghJob `json:"jobs"`
			}
			if err := g.get(fmt.Sprintf("https://api.github.com/repos/%s/actions/runs/%d/jobs?per_page=50", project, run.ID), &jobs); err != nil {
				continue
			}
			calls++
			active++
			pr := g.prNumber(project, run) // resolve once per run, not per job
			for _, job := range jobs.Jobs {
				if job.RunnerName == "" {
					continue
				}
				next[job.RunnerName] = ghPR{PR: pr, RunURL: run.HTMLURL, JobURL: job.HTMLURL, Branch: run.HeadBranch, Workflow: workflowName(run.Path)}
			}
		}
	}
	g.mu.Lock()
	g.byRunner = next
	g.mu.Unlock()
}

// prNumber resolves a run's PR by fallback order (ADR 0045): the run's
// own pull_requests, commits/{sha}/pulls, then a head-branch lookup —
// the run's field is notoriously sparse for pull_request events.
func (g *ghClient) prNumber(project string, run ghRun) int {
	if len(run.PullRequests) > 0 && run.PullRequests[0].Number > 0 {
		return run.PullRequests[0].Number
	}
	var pulls []ghPull
	if err := g.get("https://api.github.com/repos/"+project+"/commits/"+run.HeadSHA+"/pulls", &pulls); err == nil && len(pulls) > 0 && pulls[0].Number > 0 {
		return pulls[0].Number
	}
	org, _, _ := strings.Cut(project, "/")
	var byHead []ghPull
	if err := g.get("https://api.github.com/repos/"+project+"/pulls?head="+org+":"+url.QueryEscape(run.HeadBranch)+"&per_page=1", &byHead); err == nil && len(byHead) > 0 {
		return byHead[0].Number
	}
	return 0
}

// lookup maps a task workload to its PR info by runner name: the kind
// prefix drops and a "-workflow" suffix (ARC workflow pods) strips to the
// runner pod name the jobs API reports.
func (g *ghClient) lookup(workload string) (ghPR, bool) {
	name := workload
	if i := strings.Index(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimSuffix(name, "-workflow")
	g.mu.RLock()
	defer g.mu.RUnlock()
	p, ok := g.byRunner[name]
	return p, ok
}

// workflowName trims a workflow path to its file stem.
func workflowName(path string) string {
	name := strings.TrimPrefix(path, ".github/workflows/")
	return strings.TrimSuffix(strings.TrimSuffix(name, ".yaml"), ".yml")
}
