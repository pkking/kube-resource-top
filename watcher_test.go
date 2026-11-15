package main

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func TestSnapshotResolvesWorkloadOwnerChain(t *testing.T) {
	controller := true
	owned := func(kind, name string) []metav1.OwnerReference {
		return []metav1.OwnerReference{{Kind: kind, Name: name, Controller: &controller}}
	}
	s := &watchState{pods: map[string]pod{
		"n/lws-pod":    {Namespace: "n", Name: "lws-pod", OwnerRefs: owned("StatefulSet", "train-worker")},
		"n/deploy-pod": {Namespace: "n", Name: "deploy-pod", OwnerRefs: owned("ReplicaSet", "app-abc123")},
		"n/cron-pod":   {Namespace: "n", Name: "cron-pod", OwnerRefs: owned("Job", "migrate-282")},
		"n/sts-pod":    {Namespace: "n", Name: "sts-pod", OwnerRefs: owned("StatefulSet", "db")},
	}, nodes: map[string]corev1.Node{}}
	// owner objects arrive after their pods: snapshot-time resolution must roll up
	s.setOwner("StatefulSet", &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "train-worker", OwnerReferences: owned("LeaderWorkerSet", "train")}})
	s.setOwner("ReplicaSet", &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "app-abc123", OwnerReferences: owned("Deployment", "app")}})
	s.setOwner("Job", &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "migrate-282", OwnerReferences: owned("CronJob", "nightly")}})
	want := map[string]string{
		"lws-pod":    "LeaderWorkerSet/train",
		"deploy-pod": "Deployment/app",
		"cron-pod":   "CronJob/nightly",
		"sts-pod":    "StatefulSet/db", // no owner entry: stays one level deep
	}
	snap := s.snapshot()
	for _, p := range snap.Pods {
		if p.Workload != want[p.Name] {
			t.Fatalf("%s workload=%q, want %q", p.Name, p.Workload, want[p.Name])
		}
	}
	s.deleteOwner("StatefulSet", &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "train-worker"}})
	if wl := workloadOf(s.snapshot().Pods, "lws-pod"); wl != "StatefulSet/train-worker" {
		t.Fatalf("deleted owner must fall back one level, got %q", wl)
	}
}

func workloadOf(pods []pod, name string) string {
	for _, p := range pods {
		if p.Name == name {
			return p.Workload
		}
	}
	return ""
}

func TestPendingRequestUsesOnlyClusterAlias(t *testing.T) {
	source, alias := corev1.ResourceName("huawei.com/ascend-1980"), corev1.ResourceName("910C")
	s := &watchState{aliases: []aliasRule{{Alias: string(alias), Resource: string(source), Labels: map[string]string{"model": "910C"}, Unit: 2}}, pods: map[string]pod{"n/p": {Namespace: "n", Name: "p", Phase: corev1.PodPending, Requests: qtys{source: resource.MustParse("2")}}}, nodes: map[string]corev1.Node{"node": {ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"model": "910C"}}, Status: corev1.NodeStatus{Allocatable: corev1.ResourceList{source: resource.MustParse("8")}}}}}
	snapshot := s.snapshot()
	quantity := snapshot.Pods[0].Requests[alias]
	if got := quantity.Value(); got != 1 {
		t.Fatalf("alias request=%d", got)
	}
}

func TestSnapshotSumsNodeCapacityTotalWithAliasScaling(t *testing.T) {
	source, alias := corev1.ResourceName("huawei.com/ascend-1980"), corev1.ResourceName("910C")
	s := &watchState{aliases: []aliasRule{{Alias: string(alias), Resource: string(source), Labels: map[string]string{"model": "910C"}, Unit: 2}}, pods: map[string]pod{}, nodes: map[string]corev1.Node{"node": {ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"model": "910C"}}, Status: corev1.NodeStatus{Capacity: corev1.ResourceList{source: resource.MustParse("10")}, Allocatable: corev1.ResourceList{source: resource.MustParse("8")}}}}}
	snap := s.snapshot()
	total := snap.Total[alias]
	if got := total.MilliValue(); got != 5000 {
		t.Fatalf("total=%dm, want 5000m (10 over unit 2)", got)
	}
	allocatable := snap.Capacity[alias]
	if got := allocatable.MilliValue(); got != 4000 {
		t.Fatalf("allocatable=%dm, want 4000m", got)
	}
}

func TestPendingRequestStaysRawForMultipleClusterAliases(t *testing.T) {
	source := corev1.ResourceName("huawei.com/ascend-1980")
	s := &watchState{aliases: []aliasRule{{Alias: "910B", Resource: string(source), Labels: map[string]string{"model": "B"}}, {Alias: "910C", Resource: string(source), Labels: map[string]string{"model": "C"}}}, pods: map[string]pod{"n/p": {Namespace: "n", Name: "p", Phase: corev1.PodPending, Requests: qtys{source: resource.MustParse("1")}}}, nodes: map[string]corev1.Node{"b": {ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"model": "B"}}, Status: corev1.NodeStatus{Allocatable: corev1.ResourceList{source: resource.MustParse("8")}}}, "c": {ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"model": "C"}}, Status: corev1.NodeStatus{Allocatable: corev1.ResourceList{source: resource.MustParse("8")}}}}}
	snapshot := s.snapshot()
	quantity := snapshot.Pods[0].Requests[source]
	if got := quantity.Value(); got != 1 {
		t.Fatalf("raw request=%d", got)
	}
}

func TestPendingNPURequestLabelsContributeDemand(t *testing.T) {
	requests := pendingNPURequests(qtys{}, map[string]string{"ascend-ci.com/npu-resource-model": "ascend-1980", "ascend-ci.com/required-npu-count": "1"})
	quantity := requests[corev1.ResourceName("huawei.com/ascend-1980")]
	if got := quantity.Value(); got != 1 {
		t.Fatalf("request=%d", got)
	}
}

func TestPendingNPURequestLabelsDoNotDoubleCountOrAcceptInvalidCounts(t *testing.T) {
	resourceName := corev1.ResourceName("huawei.com/ascend-1980")
	requests := pendingNPURequests(qtys{resourceName: resource.MustParse("2")}, map[string]string{"ascend-ci.com/npu-resource-model": "ascend-1980", "ascend-ci.com/required-npu-count": "1"})
	quantity := requests[resourceName]
	if got := quantity.Value(); got != 2 {
		t.Fatalf("request=%d", got)
	}
	requests = pendingNPURequests(qtys{}, map[string]string{"ascend-ci.com/npu-resource-model": "ascend-1980", "ascend-ci.com/required-npu-count": "bad"})
	if len(requests) != 0 {
		t.Fatalf("invalid count added request: %#v", requests)
	}
}

func TestWatchStateTracksOnlyScheduledDemand(t *testing.T) {
	s := &watchState{pods: map[string]pod{}, nodes: map[string]corev1.Node{}}
	s.setPod(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "done"}, Status: corev1.PodStatus{Phase: corev1.PodSucceeded}})
	s.setPod(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "running", Labels: map[string]string{"ascend-ci.com/npu-resource-model": "ascend-1980", "ascend-ci.com/required-npu-count": "1"}}, Status: corev1.PodStatus{Phase: corev1.PodRunning}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}}}}}})
	snapshot := s.snapshot()
	if got := len(snapshot.Pods); got != 1 {
		t.Fatalf("pods=%d, want 1", got)
	}
	quantity := snapshot.Pods[0].Requests[corev1.ResourceName("huawei.com/ascend-1980")]
	if !quantity.IsZero() {
		t.Fatal("Running Pod must not use Pending-only labels")
	}
}

func TestProviderSideLiqoPodsAreLocalDemand(t *testing.T) {
	// Real guiyang-005 case: no virtual node exists, yet Liqo ShadowPod
	// reflections and gang-queued annotated StatefulSet (LWS) pods consume
	// and await local devices. Annotations must not hide them.
	ann := map[string]string{"liqo.io/api-server-support": "remote"}
	controller := true
	s := &watchState{target: target{ID: "c"}, pods: map[string]pod{}, nodes: map[string]corev1.Node{"node-a": {ObjectMeta: metav1.ObjectMeta{Name: "node-a"}, Status: corev1.NodeStatus{Allocatable: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4")}}}}}
	s.setPod(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "shadow", Namespace: "ns", Annotations: ann, OwnerReferences: []metav1.OwnerReference{{Kind: "ShadowPod", Name: "shadow", Controller: &controller}}}, Spec: corev1.PodSpec{NodeName: "node-a"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}})
	s.setPod(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "gang", Namespace: "ns", Annotations: ann, OwnerReferences: []metav1.OwnerReference{{Kind: "StatefulSet", Name: "lws", Controller: &controller}}}, Status: corev1.PodStatus{Phase: corev1.PodPending}})
	for _, p := range s.snapshot().Pods {
		if p.Offloaded {
			t.Fatalf("%s: provider-side pod must stay local demand", p.Name)
		}
	}
	// Same pods in a peering cluster: only the unscheduled non-shadow pod
	// waits for offload; the ShadowPod reflection stays local.
	s.nodes["liqo-remote"] = corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "liqo-remote", Labels: map[string]string{"liqo.io/type": "virtual-node"}}}
	for _, p := range s.snapshot().Pods {
		if p.Name == "gang" && !p.Offloaded {
			t.Fatal("annotated unscheduled pod in a peering cluster should be offloaded")
		}
		if p.Name == "shadow" && p.Offloaded {
			t.Fatal("ShadowPod reflection stays local demand")
		}
	}
}

func TestSnapshotMarksLiqoVirtualNodePodsAsOffloaded(t *testing.T) {
	realNode := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}, Status: corev1.NodeStatus{Allocatable: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4")}}}
	vNode := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "liqo-remote", Labels: map[string]string{"liqo.io/type": "virtual-node"}}, Status: corev1.NodeStatus{Allocatable: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("8")}}}
	s := &watchState{target: target{ID: "c"}, pods: map[string]pod{}, nodes: map[string]corev1.Node{"node-a": *realNode, "liqo-remote": *vNode}}
	s.setPod(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p-real", Namespace: "ns"}, Spec: corev1.PodSpec{NodeName: "node-a"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}})
	s.setPod(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p-offload", Namespace: "ns"}, Spec: corev1.PodSpec{NodeName: "liqo-remote"}, Status: corev1.PodStatus{Phase: corev1.PodPending}})
	snap := s.snapshot()
	var real, offload pod
	for _, p := range snap.Pods {
		switch p.Name {
		case "p-real":
			real = p
		case "p-offload":
			offload = p
		}
	}
	if real.Offloaded {
		t.Fatal("real-node pod should not be offloaded")
	}
	if !offload.Offloaded {
		t.Fatal("virtual-node pod should be marked offloaded")
	}
}

func TestSnapshotMarksLiqoAnnotatedUnscheduledPodsAsOffloaded(t *testing.T) {
	// Real cn12 case (consumer with virtual nodes): unscheduled pending
	// runner pods carry a liqo.io/ annotation and wait to be offloaded.
	s := &watchState{target: target{ID: "c"}, pods: map[string]pod{}, nodes: map[string]corev1.Node{
		"node-a":      {ObjectMeta: metav1.ObjectMeta{Name: "node-a"}, Status: corev1.NodeStatus{Allocatable: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4")}}},
		"liqo-remote": {ObjectMeta: metav1.ObjectMeta{Name: "liqo-remote", Labels: map[string]string{"liqo.io/type": "virtual-node"}}},
	}}
	s.setPod(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p-offload", Namespace: "ns", Annotations: map[string]string{"liqo.io/api-server-support": "remote"}}, Status: corev1.PodStatus{Phase: corev1.PodPending}})
	s.setPod(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p-local", Namespace: "ns"}, Spec: corev1.PodSpec{NodeName: "node-a"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}})
	snap := s.snapshot()
	var offload, local pod
	for _, p := range snap.Pods {
		switch p.Name {
		case "p-offload":
			offload = p
		case "p-local":
			local = p
		}
	}
	if !offload.Offloaded {
		t.Fatal("liqo-annotated unscheduled pod should be offloaded")
	}
	if local.Offloaded {
		t.Fatal("local pod should not be offloaded")
	}
}

func TestDeleteHandlesTombstones(t *testing.T) {
	s := &watchState{pods: map[string]pod{"ns/done": {Namespace: "ns", Name: "done", Phase: corev1.PodRunning}}, nodes: map[string]corev1.Node{"node-a": {ObjectMeta: metav1.ObjectMeta{Name: "node-a"}}}}
	s.deletePod(cache.DeletedFinalStateUnknown{Key: "ns/done", Obj: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "done"}}})
	s.deleteNode(cache.DeletedFinalStateUnknown{Key: "node-a", Obj: &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}}})
	if len(s.pods) != 0 || len(s.nodes) != 0 {
		t.Fatalf("tombstone delete ignored: pods=%d nodes=%d", len(s.pods), len(s.nodes))
	}
}

func TestSendSnapshotDeliversOrAbortsOnCancel(t *testing.T) {
	ch := make(chan resultMsg, 1)
	sendSnapshot(context.Background(), ch, snapshot{Target: target{ID: "c"}})
	if got := <-ch; got.s.Target.ID != "c" {
		t.Fatalf("snapshot not delivered: %#v", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		sendSnapshot(ctx, make(chan resultMsg), snapshot{})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("sendSnapshot must abort on canceled context, blocked instead")
	}
}
