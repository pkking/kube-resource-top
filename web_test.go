package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func qty(n int64) resource.Quantity { return *resource.NewQuantity(n, resource.DecimalSI) }

func TestWebQueueGroupsAndOrder(t *testing.T) {
	r := corev1.ResourceName("huawei.com/ascend-910b")
	base := time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC)
	s := snapshot{Pods: []pod{
		{Namespace: "ml", Name: "a-1", Workload: "LeaderWorkerSet/train-a", Phase: corev1.PodPending, Requests: qtys{r: qty(8)}, CreatedAt: base},
		{Namespace: "ml", Name: "a-2", Workload: "LeaderWorkerSet/train-a", Phase: corev1.PodPending, Requests: qtys{r: qty(8)}, CreatedAt: base.Add(time.Hour)},
		{Namespace: "dev", Name: "b-1", Workload: "Job/dist-b", Phase: corev1.PodPending, Requests: qtys{r: qty(2)}, CreatedAt: base.Add(-time.Hour)},
		{Namespace: "ml", Name: "z-0", Workload: "Job/unknown-age", Phase: corev1.PodPending, Requests: qtys{r: qty(1)}},
		{Namespace: "ml", Name: "r-1", Workload: "Job/run", Phase: corev1.PodRunning, NodeName: "n1", Requests: qtys{r: qty(4)}, CreatedAt: base},
		{Namespace: "ml", Name: "o-1", Workload: "Job/off", Phase: corev1.PodPending, Offloaded: true, Requests: qtys{r: qty(1)}, CreatedAt: base},
		{Namespace: "ml", Name: "z-1", Workload: "Job/zero", Phase: corev1.PodPending, Requests: qtys{corev1.ResourceCPU: qty(1)}, CreatedAt: base},
		{Namespace: "ml", Name: "z-2", Workload: "Job/zero-noage", Phase: corev1.PodPending, Requests: qtys{r: qty(3)}},
	}}
	q := webQueue(s, r)
	if len(q) != 4 {
		t.Fatalf("want 4 groups (running, offloaded, and zero-demand pods excluded), got %d", len(q))
	}
	if q[0].Workload != "Job/dist-b" || q[0].Position != 1 || q[0].Demand != 2 || q[0].OldestSince != base.Add(-time.Hour).UnixMilli() {
		t.Errorf("head = %+v, want Job/dist-b first with demand 2", q[0])
	}
	if q[1].Workload != "LeaderWorkerSet/train-a" || q[1].Position != 2 || q[1].Pods != 2 || q[1].Demand != 16 || q[1].LargestPod != 8 || q[1].OldestSince != base.UnixMilli() {
		t.Errorf("group 2 = %+v, want train-a pods=2 demand=16 largest=8 oldest=base", q[1])
	}
	// Unknown creation timestamps never claim the head position; ties keep name order.
	if q[2].Workload != "Job/unknown-age" || q[2].OldestSince != 0 || q[3].Workload != "Job/zero-noage" || q[3].Demand != 3 {
		t.Errorf("unknown-age groups must sort last by name, got %+v then %+v", q[2], q[3])
	}
}

func TestWebVerdict(t *testing.T) {
	r := corev1.ResourceName("huawei.com/x")
	cases := []struct {
		cr   webClusterRes
		want string
	}{
		{webClusterRes{}, "empty"},
		{webClusterRes{Allocatable: 0, Queue: []webQueueGroup{{LargestPod: 2}}}, "absent"},
		{webClusterRes{Allocatable: 8, Running: 8, Free: 0, Queue: []webQueueGroup{{LargestPod: 2}}}, "full"},
		{webClusterRes{Allocatable: 16, Running: 10, Free: 6, MaxNodeFree: 2, Queue: []webQueueGroup{{LargestPod: 4}}}, "fragmented"},
		{webClusterRes{Allocatable: 16, Running: 10, Free: 6, MaxNodeFree: 6, Queue: []webQueueGroup{{LargestPod: 4}}}, "blocked"},
	}
	for _, c := range cases {
		if got, _ := webVerdict(r, c.cr); got != c.want {
			t.Errorf("verdict(%+v) = %s, want %s", c.cr, got, c.want)
		}
	}
}

func TestWebQtyAndUnits(t *testing.T) {
	cpu := *resource.NewMilliQuantity(250, resource.DecimalSI)
	if got := webQty(corev1.ResourceCPU, cpu); got != 0.25 {
		t.Errorf("cpu 250m = %v, want 0.25", got)
	}
	mem := *resource.NewQuantity(5*1024*1024*1024, resource.BinarySI)
	if got := webQty(corev1.ResourceMemory, mem); got != 5 {
		t.Errorf("memory 5Gi = %v, want 5", got)
	}
	if got := webQty("huawei.com/ascend-1980", qty(8)); got != 8 {
		t.Errorf("cards = %v, want 8", got)
	}
	if got := webQtyText("huawei.com/ascend-1980", 8); got != "8 张" {
		t.Errorf("text = %q, want 8 张", got)
	}
	if got := webQtyText("huawei.com/ascend-1980", 1.5); got != "1.5 张" {
		t.Errorf("text = %q, want 1.5 张", got)
	}
	if got := webQtyText(corev1.ResourceCPU, 2); got != "2 核" {
		t.Errorf("text = %q, want 2 核", got)
	}
}

func TestWebStateShapesAccounting(t *testing.T) {
	r := corev1.ResourceName("huawei.com/ascend-910b")
	now := time.Now()
	app := &webApp{
		targets: []target{
			{ID: "a", Path: "/kube/cn1.yaml", Context: "cn1"},
			{ID: "b", Path: "/kube/cn2.yaml", Context: "cn2"},
			{ID: "c", Path: "/kube/cn3.yaml", Context: "cn3"},
		},
		snaps: map[string]snapshot{
			"a": {Target: target{ID: "a", Path: "/kube/cn1.yaml", Context: "cn1"},
				Capacity: qtys{r: qty(16)}, Total: qtys{r: qty(32)},
				Nodes: []nodeInfo{{Name: "n1", Capacity: qtys{r: qty(16)}}},
				Pods: []pod{
					{Namespace: "ml", Name: "run-1", Workload: "Job/run", Phase: corev1.PodRunning, NodeName: "n1", Requests: qtys{r: qty(8)}, CreatedAt: now.Add(-time.Hour)},
					{Namespace: "ml", Name: "wait-1", Workload: "Job/wait", Phase: corev1.PodPending, Requests: qtys{r: qty(8)}, CreatedAt: now.Add(-time.Minute)},
				}},
			// Zero-allocatable cluster with queued demand: excluded from the
			// aggregates (ADR 0013) but surfaced as an explicit absent row. The
			// watcher always initializes Capacity, so an empty non-nil map means
			// "reports nothing for this resource".
			"b": {Target: target{ID: "b", Path: "/kube/cn2.yaml", Context: "cn2"}, Capacity: qtys{}, Total: qtys{},
				Pods: []pod{{Namespace: "dev", Name: "orphan-1", Workload: "Job/orphan", Phase: corev1.PodPending, Requests: qtys{r: qty(4)}, CreatedAt: now}}},
		},
		resourceSelected: map[corev1.ResourceName]bool{r: true},
		selected:         map[string]bool{"a": true, "b": true, "c": true},
	}
	st := app.state()
	if st.Live != 2 || st.Total != 3 {
		t.Fatalf("live/total = %d/%d, want 2/3 (c has no snapshot)", st.Live, st.Total)
	}
	if len(st.Clusters) != 3 || st.Clusters[2].ID != "c" || st.Clusters[2].Err == "" {
		t.Fatalf("cluster list = %+v, want c listed with a connection error", st.Clusters)
	}
	if len(st.Resources) != 1 {
		t.Fatalf("want 1 resource, got %d", len(st.Resources))
	}
	res := st.Resources[0]
	if res.Name != string(r) || res.Total != 32 || res.Allocatable != 16 || res.Running != 8 || res.Pending != 8 || res.Ratio != 1 {
		t.Errorf("aggregate = %+v, want total 32 alloc 16 running 8 pending 8 ratio 1", res)
	}
	if len(res.Clusters) != 2 {
		t.Fatalf("want 2 cluster rows, got %d", len(res.Clusters))
	}
	first, second := res.Clusters[0], res.Clusters[1]
	if first.ClusterID != "a" || first.Free != 8 || first.FreeNodes != 1 || first.MaxNodeFree != 8 {
		t.Errorf("cluster a = %+v, want free 8 on 1 node (max 8)", first)
	}
	if len(first.Queue) != 1 || first.Queue[0].Workload != "Job/wait" || first.Queue[0].Position != 1 || first.Queue[0].Demand != 8 {
		t.Errorf("cluster a queue = %+v, want Job/wait at position 1", first.Queue)
	}
	if first.Verdict != "blocked" {
		t.Errorf("cluster a verdict = %s, want blocked (a fitting node exists)", first.Verdict)
	}
	if second.ClusterID != "b" || !second.Absent || second.Pending != 4 || second.Verdict != "absent" {
		t.Errorf("cluster b = %+v, want an absent row with pending 4", second)
	}
	if res.PendingWorkloads != 2 {
		t.Errorf("pendingWorkloads = %d, want 2", res.PendingWorkloads)
	}
}

// The page indexes .length on every array field, so a nil slice marshaling as
// null would wedge the first render (observed live on an absent cluster row).
func TestWebStateJSONHasNoNullArrays(t *testing.T) {
	r := corev1.ResourceName("huawei.com/ascend-910b")
	app := &webApp{
		targets: []target{{ID: "a", Path: "/kube/cn1.yaml", Context: "cn1"}},
		snaps: map[string]snapshot{
			// Pending-only cluster: allocatable 0, no nodes — the nil-slice trap.
			"a": {Target: target{ID: "a", Path: "/kube/cn1.yaml", Context: "cn1"}, Capacity: qtys{}, Total: qtys{},
				Pods: []pod{{Namespace: "dev", Name: "orphan-1", Workload: "Job/orphan", Phase: corev1.PodPending, Requests: qtys{r: qty(4)}}}},
		},
		resourceSelected: map[corev1.ResourceName]bool{r: true},
		selected:         map[string]bool{"a": true},
	}
	b, err := json.Marshal(app.state())
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, bad := range []string{`"nodes":null`, `"clusters":null`, `"queue":null`, `"resources":null`} {
		if strings.Contains(s, bad) {
			t.Errorf("marshaled state contains %s; the page cannot render null arrays", bad)
		}
	}
}
