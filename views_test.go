package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestNodeBalanceShowsAllocatableAndRunningRequests(t *testing.T) {
	gpu := corev1.ResourceName("example.com/gpu")
	m := model{resource: gpu, snaps: map[string]snapshot{"c": {Target: target{ID: "c", Context: "cluster"}, Nodes: []nodeInfo{{Name: "node-a", IP: "10.0.0.1", Capacity: qtys{gpu: resource.MustParse("8")}}, {Name: "node-b", Capacity: qtys{gpu: resource.MustParse("0")}}}, Pods: []pod{{NodeName: "node-a", Phase: corev1.PodRunning, Requests: qtys{gpu: resource.MustParse("2")}}, {NodeName: "node-a", Phase: corev1.PodPending, Requests: qtys{gpu: resource.MustParse("3")}}}}}}
	rows := m.nodeRows()
	if len(rows) != 1 || rows[0].name != "10.0.0.1" || rows[0].capacity.Value() != 8 || rows[0].requested.Value() != 2 {
		t.Fatalf("rows=%#v", rows)
	}
}

func TestNodePodsCountsRunningRequestsAndSkipsOthers(t *testing.T) {
	gpu := corev1.ResourceName("example.com/gpu")
	m := model{resource: gpu, snaps: map[string]snapshot{"c": {Target: target{ID: "c"}, Nodes: []nodeInfo{{Name: "node-a", Capacity: qtys{gpu: resource.MustParse("8")}}}, Pods: []pod{
		{Namespace: "ns", Name: "p1", NodeName: "node-a", Phase: corev1.PodRunning, Requests: qtys{gpu: resource.MustParse("2")}},
		{Namespace: "ns", Name: "p2", NodeName: "node-a", Phase: corev1.PodRunning, Requests: qtys{gpu: resource.MustParse("3")}},
		{Namespace: "ns", Name: "pending", NodeName: "node-a", Phase: corev1.PodPending, Requests: qtys{gpu: resource.MustParse("5")}},
		{Namespace: "ns", Name: "other", NodeName: "node-b", Phase: corev1.PodRunning, Requests: qtys{gpu: resource.MustParse("1")}},
	}}}}
	pods := m.nodePods("c", "node-a", gpu)
	if len(pods) != 2 {
		t.Fatalf("pods=%d, want 2", len(pods))
	}
	var sum int64
	for _, p := range pods {
		req := p.Requests[gpu]
		sum += req.Value()
	}
	if sum != 5 {
		t.Fatalf("sum=%d, want 5", sum)
	}
}

func TestResourceAggregateViewGroupsContexts(t *testing.T) {
	cpu := corev1.ResourceCPU
	m := model{viewMode: 2, resourceSelected: map[corev1.ResourceName]bool{cpu: true}, snaps: map[string]snapshot{
		"a": {Target: target{ID: "a", Context: "a"}, Pods: []pod{{Requests: qtys{cpu: resource.MustParse("1")}, Usage: qtys{cpu: resource.MustParse("1")}}}},
		"b": {Target: target{ID: "b", Context: "b"}, Pods: []pod{{Requests: qtys{cpu: resource.MustParse("2")}, Usage: qtys{cpu: resource.MustParse("1")}}}},
	}}
	rows := m.viewRows()
	if len(rows) != 1 {
		t.Fatalf("rows=%d, want 1", len(rows))
	}
	q := rows[0].q
	if got := q.Value(); got != 3 {
		t.Fatalf("request=%d, want 3", got)
	}
}

func TestAggregateViewShowsTotalAndOccupancy(t *testing.T) {
	gpu := corev1.ResourceName("example.com/gpu")
	m := model{resource: gpu, resourceSelected: map[corev1.ResourceName]bool{gpu: true}, snaps: map[string]snapshot{"c": {Target: target{ID: "c", Context: "cluster"}, Total: qtys{gpu: resource.MustParse("8")}, Capacity: qtys{gpu: resource.MustParse("6")}, Pods: []pod{
		{Namespace: "ns", Name: "run", NodeName: "node-a", Phase: corev1.PodRunning, Requests: qtys{gpu: resource.MustParse("3")}},
		{Namespace: "ns", Name: "pend", Phase: corev1.PodPending, Requests: qtys{gpu: resource.MustParse("1")}},
	}}}}
	out := m.aggregateView()
	want := "total 8  req/alloc 67%  running 3  pending 1  allocatable 6"
	if !strings.Contains(out, want) {
		t.Fatalf("aggregate view missing %q:\n%s", want, out)
	}
	if strings.Contains(out, "N/A") {
		t.Fatalf("aggregate view must not show the usage pair:\n%s", out)
	}
}

func TestNameWidthFitsLongestBoundedByTerminal(t *testing.T) {
	long := "short-name-here-12345678901234567890" // 36 chars
	m := model{width: 120}
	if got := m.nameWidth([]string{"a", long}, 40, 10); got != 36 {
		t.Fatalf("got %d, want 36", got)
	}
	// narrow terminal caps the column instead of overflowing
	if got := (model{width: 60}).nameWidth([]string{"a", "very-very-long-name-that-exceeds-terminal"}, 40, 10); got != 20 {
		t.Fatalf("got %d, want 20", got)
	}
}

func TestBFromDrilledClusterShowsOnlyItsNodes(t *testing.T) {
	gpu := corev1.ResourceName("example.com/gpu")
	m := model{resource: gpu, width: 200, height: 40, snaps: map[string]snapshot{
		"c": {Target: target{ID: "c", Context: "cluster"}, Nodes: []nodeInfo{{Name: "node-c", Capacity: qtys{gpu: resource.MustParse("8")}}}},
		"d": {Target: target{ID: "d", Context: "clusterD"}, Nodes: []nodeInfo{{Name: "node-d", Capacity: qtys{gpu: resource.MustParse("8")}}}},
	}}
	m.selected = map[string]bool{"c": true, "d": true}
	m.scope = []string{"c"}
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	mm := u.(model)
	if !mm.nodeBalance || len(mm.nodeRows()) != 1 || mm.nodeRows()[0].nodeName != "node-c" {
		t.Fatalf("b should show only drilled context nodes: %#v", mm.nodeRows())
	}
}

func TestDrillSelectsRowResourceBeforeOpeningNodes(t *testing.T) {
	gpu, npu := corev1.ResourceName("example.com/gpu"), corev1.ResourceName("example.com/npu")
	m := model{resource: corev1.ResourceCPU, resourceSelected: map[corev1.ResourceName]bool{gpu: true, npu: true}, snaps: map[string]snapshot{
		"c": {Target: target{ID: "c"}, Pods: []pod{{Requests: qtys{gpu: resource.MustParse("1"), npu: resource.MustParse("1")}}}},
	}}
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(model)
	if m.resource != gpu || len(m.scope) != 1 || m.scope[0] != "c" {
		t.Fatalf("drill=%#v resource=%s", m.scope, m.resource)
	}
}

func TestNodeViewScopeModes(t *testing.T) {
	gpu, npu := corev1.ResourceName("example.com/gpu"), corev1.ResourceName("example.com/npu")
	m := model{resource: gpu, scope: []string{"a"}, selected: map[string]bool{"a": true, "b": true}, resourceSelected: map[corev1.ResourceName]bool{gpu: true, npu: true}, snaps: map[string]snapshot{
		"a": {Target: target{ID: "a", Context: "a"}, Nodes: []nodeInfo{{Name: "node-a", Capacity: qtys{gpu: resource.MustParse("8"), npu: resource.MustParse("4")}}}},
		"b": {Target: target{ID: "b", Context: "b"}, Nodes: []nodeInfo{{Name: "node-b", Capacity: qtys{gpu: resource.MustParse("8"), npu: resource.MustParse("4")}}}},
	}}
	if rows := m.nodeRows(); len(rows) != 1 || rows[0].nodeName != "node-a" || rows[0].resource != gpu {
		t.Fatalf("drill rows=%#v", rows)
	}
	m.nodeScope = nodeScopeAll
	if rows := m.nodeRows(); len(rows) != 4 {
		t.Fatalf("all rows=%#v", rows)
	}
	m.nodeScope = nodeScopeContexts
	if rows := m.nodeRows(); len(rows) != 2 || rows[0].resource != gpu || rows[1].resource != gpu {
		t.Fatalf("context rows=%#v", rows)
	}
	m.nodeScope = nodeScopeResources
	if rows := m.nodeRows(); len(rows) != 4 {
		t.Fatalf("resource rows=%#v", rows)
	}
}

func TestEscExitsNodeBalanceView(t *testing.T) {
	gpu := corev1.ResourceName("example.com/gpu")
	m := model{resource: gpu, width: 200, height: 40, snaps: map[string]snapshot{"c": {Target: target{ID: "c", Context: "cluster"}, Nodes: []nodeInfo{{Name: "node-a", Capacity: qtys{gpu: resource.MustParse("8")}}}, Pods: []pod{{Namespace: "ns", Name: "p1", NodeName: "node-a", Phase: corev1.PodRunning, Requests: qtys{gpu: resource.MustParse("2")}}}}}}
	m.nodeBalance = true
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if mm := u.(model); mm.nodeBalance {
		t.Fatal("esc should exit the nodes view")
	}
}

func TestNodeViewShowsPendingAndPerPodStatus(t *testing.T) {
	gpu := corev1.ResourceName("example.com/gpu")
	m := model{resource: gpu, width: 220, height: 40, snaps: map[string]snapshot{"c": {Target: target{ID: "c", Context: "cluster"}, Nodes: []nodeInfo{{Name: "node-a", IP: "10.0.0.1", Capacity: qtys{gpu: resource.MustParse("8")}}}, Pods: []pod{
		{Namespace: "ns", Name: "running-pod", NodeName: "node-a", Phase: corev1.PodRunning, Requests: qtys{gpu: resource.MustParse("2")}},
		{Namespace: "ns", Name: "pending-pod", NodeName: "", Phase: corev1.PodPending, Requests: qtys{gpu: resource.MustParse("3")}},
	}}}}
	m.nodeBalance = true
	m.cursor = 0
	out := m.View()
	if !strings.Contains(out, "PENDING") {
		t.Fatalf("table missing PENDING column")
	}
	if !strings.Contains(out, "running: 1") || !strings.Contains(out, "pending: 1") {
		t.Fatalf("frag panel missing counts:\n%s", out)
	}
	if !strings.Contains(out, "RUNNING") {
		t.Fatalf("frag panel missing RUNNING status")
	}
	if !strings.Contains(out, "ns/pending-pod") {
		t.Fatalf("frag panel missing pending pod")
	}
}

func TestBEntersNodeViewOnSelectedResource(t *testing.T) {
	gpu1 := corev1.ResourceName("a1")
	gpu2 := corev1.ResourceName("a2")
	m := model{resource: corev1.ResourceCPU, resourceSelected: map[corev1.ResourceName]bool{gpu1: true, gpu2: true}, width: 200, height: 40, snaps: map[string]snapshot{"c": {Target: target{ID: "c", Context: "cluster"}, Capacity: qtys{gpu1: resource.MustParse("8"), gpu2: resource.MustParse("4")}, Nodes: []nodeInfo{{Name: "node-a", Capacity: qtys{gpu1: resource.MustParse("8"), gpu2: resource.MustParse("4")}}}}}}
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	mm := u.(model)
	if mm.resource != gpu1 {
		t.Fatalf("b should enter node view on first selected resource, got %v", mm.resource)
	}
}

func TestTabCyclesSelectedResourcesInNodeView(t *testing.T) {
	gpu1 := corev1.ResourceName("a1")
	gpu2 := corev1.ResourceName("a2")
	m := model{resource: gpu1, nodeBalance: true, resourceSelected: map[corev1.ResourceName]bool{gpu1: true, gpu2: true}, width: 200, height: 40, snaps: map[string]snapshot{"c": {Target: target{ID: "c", Context: "cluster"}, Capacity: qtys{gpu1: resource.MustParse("8"), gpu2: resource.MustParse("4")}, Nodes: []nodeInfo{{Name: "node-a", Capacity: qtys{gpu1: resource.MustParse("8"), gpu2: resource.MustParse("4")}}}}}}
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	mm := u.(model)
	if mm.resource != gpu2 {
		t.Fatalf("tab should cycle to a2, got %v", mm.resource)
	}
}

func TestOffloadedPendingPodsExcludedFromPendingCount(t *testing.T) {
	gpu := corev1.ResourceName("example.com/gpu")
	m := model{resource: gpu, width: 200, height: 40, snaps: map[string]snapshot{"c": {Target: target{ID: "c", Context: "cluster"}, Capacity: qtys{gpu: resource.MustParse("16")}, Nodes: []nodeInfo{{Name: "node-a", Capacity: qtys{gpu: resource.MustParse("16")}}}, Pods: []pod{
		{Namespace: "ns", Name: "local-pending", Phase: corev1.PodPending, Requests: qtys{gpu: resource.MustParse("3")}},
		{Namespace: "ns", Name: "offload-pending", NodeName: "liqo-remote", Phase: corev1.PodPending, Offloaded: true, Requests: qtys{gpu: resource.MustParse("8")}},
	}}}}
	if got := m.rows()[0].pending.Value(); got != 3 {
		t.Fatalf("pending=%d, want 3 (offloaded must be excluded)", got)
	}
	if got := m.nodeRows()[0].pending.Value(); got != 3 {
		t.Fatalf("node pending=%d, want 3 (offloaded must be excluded)", got)
	}
}

func TestContextAndPodViewsExcludeOffloadedPods(t *testing.T) {
	cpu := corev1.ResourceCPU
	m := model{resource: cpu, resourceSelected: map[corev1.ResourceName]bool{cpu: true}, width: 200, height: 40, snaps: map[string]snapshot{"c": {Target: target{ID: "c", Context: "cluster"}, Capacity: qtys{cpu: resource.MustParse("16")}, Nodes: []nodeInfo{{Name: "node-a", Capacity: qtys{cpu: resource.MustParse("16")}}}, Pods: []pod{
		{Namespace: "ns", Name: "local", Workload: "wl", NodeName: "node-a", Phase: corev1.PodRunning, Requests: qtys{cpu: resource.MustParse("2")}},
		{Namespace: "ns", Name: "offload-run", Workload: "wl", NodeName: "liqo-remote", Phase: corev1.PodRunning, Offloaded: true, Requests: qtys{cpu: resource.MustParse("8")}},
		{Namespace: "ns", Name: "offload-pend", Workload: "wl", NodeName: "liqo-remote", Phase: corev1.PodPending, Offloaded: true, Requests: qtys{cpu: resource.MustParse("4")}},
		{Namespace: "ns", Name: "local-pend", Workload: "wl", Phase: corev1.PodPending, Requests: qtys{cpu: resource.MustParse("1")}},
	}}}}
	r := m.rows()[0]
	if got := r.q.Value(); got != 3 {
		t.Fatalf("request=%d, want 3", got)
	}
	if got := r.running.Value(); got != 2 {
		t.Fatalf("running=%d, want 2", got)
	}
	if got := r.pending.Value(); got != 1 {
		t.Fatalf("pending=%d, want 1", got)
	}
	// drill to pod level: offloaded pods must not appear
	drill := model{resource: cpu, resourceSelected: map[corev1.ResourceName]bool{cpu: true}, scope: []string{"c", "ns", "wl"}, width: 200, height: 40, snaps: m.snaps}
	prows := drill.rows()
	if len(prows) != 2 {
		t.Fatalf("pod rows=%d, want 2 (offloaded excluded)", len(prows))
	}
}

// ageSuffixOnScopeHeader reports whether the header line (the line containing
// "SCOPE") ends with the AGE column, and fails the test if no header is found.
func ageSuffixOnScopeHeader(t *testing.T, out string) bool {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "SCOPE") {
			return strings.HasSuffix(strings.TrimRight(line, " "), "AGE")
		}
	}
	t.Fatal("no header line containing SCOPE")
	return false
}

func TestPodDrillDownShowsAgeColumn(t *testing.T) {
	cpu := corev1.ResourceCPU
	m := model{resource: cpu, resourceSelected: map[corev1.ResourceName]bool{cpu: true}, selected: map[string]bool{"t": true}, scope: []string{"t", "ns", "Deployment/app"}, width: 140, height: 40, snaps: map[string]snapshot{"t": {Target: target{ID: "t"}, Capacity: qtys{cpu: resource.MustParse("10")}, Pods: []pod{
		{Namespace: "ns", Name: "pod-a", Workload: "Deployment/app", Phase: corev1.PodRunning, NodeName: "node-1", Requests: qtys{cpu: resource.MustParse("1")}, CreatedAt: time.Now().Add(-70 * time.Minute)},
		{Namespace: "ns", Name: "pod-b", Workload: "Deployment/app", Phase: corev1.PodRunning, NodeName: "node-1", Requests: qtys{cpu: resource.MustParse("1")}},
	}}}}
	out := m.View()
	if !ageSuffixOnScopeHeader(t, out) {
		t.Fatalf("pod detail header should end with AGE:\n%s", out)
	}
	// aggregate views (v cycles viewMode, scope stays set) must not show AGE
	agg := m
	agg.viewMode = 1
	if ageSuffixOnScopeHeader(t, agg.View()) {
		t.Fatalf("clusters aggregate header must not end with AGE:\n%s", agg.View())
	}
	agg.viewMode = 2
	if ageSuffixOnScopeHeader(t, agg.View()) {
		t.Fatalf("resources aggregate header must not end with AGE:\n%s", agg.View())
	}
	if !strings.Contains(out, "70m") {
		t.Fatalf("pod view missing 70m age:\n%s", out)
	}
	if !strings.Contains(out, "N/A") {
		t.Fatalf("zero CreatedAt pod should render N/A:\n%s", out)
	}
}

func TestWorkloadDrillDownShowsOldestPodAge(t *testing.T) {
	cpu := corev1.ResourceCPU
	m := model{resource: cpu, resourceSelected: map[corev1.ResourceName]bool{cpu: true}, selected: map[string]bool{"t": true}, scope: []string{"t", "ns"}, width: 140, height: 40, snaps: map[string]snapshot{"t": {Target: target{ID: "t"}, Capacity: qtys{cpu: resource.MustParse("10")}, Pods: []pod{
		{Namespace: "ns", Workload: "Deployment/app", Phase: corev1.PodRunning, NodeName: "node-1", Requests: qtys{cpu: resource.MustParse("1")}, CreatedAt: time.Now().Add(-3 * time.Hour)},
		{Namespace: "ns", Workload: "Deployment/app", Phase: corev1.PodRunning, NodeName: "node-1", Requests: qtys{cpu: resource.MustParse("1")}, CreatedAt: time.Now().Add(-30 * time.Minute)},
		{Namespace: "ns", Workload: "Job/other", Phase: corev1.PodRunning, NodeName: "node-1", Requests: qtys{cpu: resource.MustParse("1")}},
	}}}}
	out := m.View()
	if !ageSuffixOnScopeHeader(t, out) {
		t.Fatalf("workload detail header should end with AGE:\n%s", out)
	}
	if !strings.Contains(out, "3h") {
		t.Fatalf("workload row should show oldest pod age 3h:\n%s", out)
	}
	if !strings.Contains(out, "N/A") {
		t.Fatalf("workload with unknown pod timestamps should render N/A:\n%s", out)
	}
	if strings.Contains(out, "30m") {
		t.Fatalf("younger pod age must not be shown (oldest pod wins):\n%s", out)
	}
	// shallower scopes must not show AGE
	cluster := m
	cluster.scope = nil
	if ageSuffixOnScopeHeader(t, cluster.View()) {
		t.Fatalf("cluster scope header must not end with AGE:\n%s", cluster.View())
	}
	nsScope := m
	nsScope.scope = []string{"t"}
	if ageSuffixOnScopeHeader(t, nsScope.View()) {
		t.Fatalf("namespace scope header must not end with AGE:\n%s", nsScope.View())
	}
}

func TestRefreshTickRearms(t *testing.T) {
	m := model{}
	u, cmd := m.Update(refreshMsg{})
	if cmd == nil {
		t.Fatal("refresh tick must re-arm")
	}
	if _, ok := u.(model); !ok {
		t.Fatalf("Update returned %T", u)
	}
}
