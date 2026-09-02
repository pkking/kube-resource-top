package main

import (
	"strings"
	"testing"

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
	pods := m.nodePods("c", "node-a")
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

func TestBFromDrilledClusterShowsNodesImmediately(t *testing.T) {
	gpu := corev1.ResourceName("example.com/gpu")
	m := model{resource: gpu, width: 200, height: 40, snaps: map[string]snapshot{
		"c": {Target: target{ID: "c", Context: "cluster"}, Nodes: []nodeInfo{{Name: "node-a", Capacity: qtys{}}}},                  // no gpu capacity
		"d": {Target: target{ID: "d", Context: "clusterD"}, Nodes: []nodeInfo{{Name: "node-d", Capacity: qtys{gpu: resource.MustParse("8")}}}},
	}}
	m.selected = map[string]bool{"c": true, "d": true}
	m.scope = []string{"c"} // drilled into the cluster that lacks the resource
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	mm := u.(model)
	if !mm.nodeBalance || len(mm.scope) != 1 {
		t.Fatalf("b should toggle nodes view but keep drill scope: nodeBalance=%v scope=%v", mm.nodeBalance, mm.scope)
	}
	if !strings.Contains(mm.View(), "node-d") {
		t.Fatalf("nodes did not appear immediately after b (scope still hides them)")
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
