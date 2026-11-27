package main

import (
	"encoding/json"
	"testing"
)

func TestGHLookupNormalizesRunnerNames(t *testing.T) {
	g := &ghClient{byRunner: map[string]ghPR{
		"linux-a3-sglang-fed-2-runner-hlpnk": {PR: 123, Branch: "feat/x", Workflow: "e2e"},
	}}
	cases := []struct {
		workload string
		want     bool
	}{
		{"Pod/linux-a3-sglang-fed-2-runner-hlpnk", true},          // runner pod
		{"Pod/linux-a3-sglang-fed-2-runner-hlpnk-workflow", true}, // ARC workflow pod
		{"Job/some-batch", false},                                 // not a runner
		{"LeaderWorkerSet/train", false},                          // not a runner
		{"Pod/linux-a3-sglang-fed-2-runner-someoneelse", false},   // different runner
	}
	for _, c := range cases {
		p, ok := g.lookup(c.workload)
		if ok != c.want || (c.want && p.PR != 123) {
			t.Errorf("lookup(%q) = (%+v, %v), want hit=%v PR=123", c.workload, p, ok, c.want)
		}
	}
}

func TestGHWorkflowName(t *testing.T) {
	cases := map[string]string{
		".github/workflows/e2e_test.yaml": "e2e_test",
		".github/workflows/ci.yml":        "ci",
		"weird.yaml":                      "weird",
	}
	for path, want := range cases {
		if got := workflowName(path); got != want {
			t.Errorf("workflowName(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestGHParseRunsWithSparsePullRequests(t *testing.T) {
	// Observed live: event=pull_request runs with an empty pull_requests
	// array — the prNumber fallbacks must carry the mapping (ADR 0045).
	var runs struct {
		WorkflowRuns []ghRun `json:"workflow_runs"`
	}
	fixture := `{"workflow_runs":[{"id":45915074172,"status":"in_progress","head_branch":"feat/deepgemm-mrv2-eplb",
		"head_sha":"f1f27eec0d","html_url":"https://github.com/o/r/actions/runs/45915074172",
		"path":".github/workflows/schedule_e2e_upstream_test.yaml","pull_requests":[]}]}`
	if err := json.Unmarshal([]byte(fixture), &runs); err != nil {
		t.Fatal(err)
	}
	if len(runs.WorkflowRuns) != 1 || runs.WorkflowRuns[0].HeadBranch != "feat/deepgemm-mrv2-eplb" {
		t.Fatalf("parsed = %+v", runs)
	}
	if len(runs.WorkflowRuns[0].PullRequests) != 0 {
		t.Fatal("sparse pull_requests must parse as empty, not error")
	}
}
