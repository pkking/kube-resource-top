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
	"sync/atomic"
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
	// steady 3-minute tick when nothing maps at all, e.g. at night).
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
	tick := time.NewTicker(3 * time.Minute)
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

// refresh sweeps widely within a fixed budget (ADR 0049): fifty
// in-progress runs and up to thirty runs' jobs per project, PR numbers
// only for runs whose jobs matched an actual board runner. Projects
// sweep in parallel (8 workers); budgets count atomically. Any API
// failure skips that piece; the cache degrades to missing entries,
// never to guessed links.
func (g *ghClient) refresh(a *webApp) {
	const budget = 200 // calls per 3-minute refresh, ≤ 4000/h
	const prBudget = 25
	projects := map[string]bool{}
	boardRunners := map[string]bool{}
	st := a.state()
	for _, res := range st.Resources {
		for _, t := range res.Tasks {
			if t.Run > 0 && strings.Contains(t.Project, "/") {
				projects[t.Project] = true
			}
			name := t.Workload
			if i := strings.Index(name, "/"); i >= 0 {
				name = name[i+1:]
			}
			boardRunners[strings.TrimSuffix(name, "-workflow")] = true
		}
	}
	var calls, prCalls int32
	next := map[string]ghPR{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for project := range projects {
		wg.Add(1)
		sem <- struct{}{}
		go func(project string) {
			defer wg.Done()
			defer func() { <-sem }()
			local := g.sweepProject(project, boardRunners, &calls, &prCalls, budget, prBudget)
			if len(local) == 0 {
				return
			}
			mu.Lock()
			for k, v := range local {
				next[k] = v
			}
			mu.Unlock()
		}(project)
	}
	wg.Wait()
	g.mu.Lock()
	g.byRunner = next
	g.mu.Unlock()
}

// sweepProject collects runner → PR info for one project (ADR 0049).
func (g *ghClient) sweepProject(project string, boardRunners map[string]bool, calls, prCalls *int32, budget, prBudget int32) map[string]ghPR {
	if atomic.LoadInt32(calls) >= budget {
		return nil
	}
	var runs struct {
		WorkflowRuns []ghRun `json:"workflow_runs"`
	}
	// Only in-progress runs carry runner names in their jobs; on a busy
	// repo they fall outside a plain recent-runs list, so filter by
	// status instead of paging through completed history.
	if err := g.get("https://api.github.com/repos/"+project+"/actions/runs?per_page=50&status=in_progress", &runs); err != nil {
		return nil
	}
	atomic.AddInt32(calls, 1)
	type matched struct {
		run ghRun
		job ghJob
	}
	jobsByRunner := map[string]matched{}
	active := 0
	for _, run := range runs.WorkflowRuns {
		if atomic.LoadInt32(calls) >= budget || active >= 30 {
			break
		}
		var jobs struct {
			Jobs []ghJob `json:"jobs"`
		}
		if err := g.get(fmt.Sprintf("https://api.github.com/repos/%s/actions/runs/%d/jobs?per_page=50", project, run.ID), &jobs); err != nil {
			continue
		}
		atomic.AddInt32(calls, 1)
		active++
		for _, job := range jobs.Jobs {
			if job.RunnerName == "" || !boardRunners[job.RunnerName] {
				continue
			}
			if _, seen := jobsByRunner[job.RunnerName]; seen {
				continue // first run wins
			}
			jobsByRunner[job.RunnerName] = matched{run: run, job: job}
		}
	}
	// PR resolution spends calls only on runs the board actually shows.
	prByRun := map[int64]int{}
	for _, m := range jobsByRunner {
		if atomic.LoadInt32(calls) >= budget || atomic.LoadInt32(prCalls) >= prBudget {
			break
		}
		if _, done := prByRun[m.run.ID]; done {
			continue
		}
		pr, used := g.prNumber(project, m.run)
		atomic.AddInt32(calls, int32(used))
		atomic.AddInt32(prCalls, int32(used))
		prByRun[m.run.ID] = pr
	}
	out := map[string]ghPR{}
	for runner, m := range jobsByRunner {
		out[runner] = ghPR{PR: prByRun[m.run.ID], RunURL: m.run.HTMLURL, JobURL: m.job.HTMLURL, Branch: m.run.HeadBranch, Workflow: workflowName(m.run.Path)}
	}
	return out
}

// prNumber resolves a run's PR by fallback order (ADR 0045): the run's
// own pull_requests, commits/{sha}/pulls, then a head-branch lookup —
// the run's field is notoriously sparse for pull_request events. It
// returns the PR number and the calls it spent, failed or not.
func (g *ghClient) prNumber(project string, run ghRun) (int, int) {
	if len(run.PullRequests) > 0 && run.PullRequests[0].Number > 0 {
		return run.PullRequests[0].Number, 0
	}
	used := 0
	var pulls []ghPull
	if err := g.get("https://api.github.com/repos/"+project+"/commits/"+run.HeadSHA+"/pulls", &pulls); err == nil {
		used++
		if len(pulls) > 0 && pulls[0].Number > 0 {
			return pulls[0].Number, used
		}
	} else {
		used++
	}
	org, _, _ := strings.Cut(project, "/")
	var byHead []ghPull
	if err := g.get("https://api.github.com/repos/"+project+"/pulls?head="+org+":"+url.QueryEscape(run.HeadBranch)+"&per_page=1", &byHead); err == nil {
		used++
		if len(byHead) > 0 {
			return byHead[0].Number, used
		}
	} else {
		used++
	}
	return 0, used
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
