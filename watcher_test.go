package main

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPendingRequestUsesOnlyClusterAlias(t *testing.T) {
	source, alias := corev1.ResourceName("huawei.com/ascend-1980"), corev1.ResourceName("910C")
	s := &watchState{aliases: []aliasRule{{Alias: string(alias), Resource: string(source), Labels: map[string]string{"model": "910C"}, Unit: 2}}, pods: map[string]pod{"n/p": {Namespace: "n", Name: "p", Phase: corev1.PodPending, Requests: qtys{source: resource.MustParse("2")}}}, nodes: map[string]corev1.Node{"node": {ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"model": "910C"}}, Status: corev1.NodeStatus{Allocatable: corev1.ResourceList{source: resource.MustParse("8")}}}}}
	snapshot := s.snapshot()
	quantity := snapshot.Pods[0].Requests[alias]
	if got := quantity.Value(); got != 1 {
		t.Fatalf("alias request=%d", got)
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
	// Real cn12 case: unscheduled pending runner pods carry a liqo.io/ annotation.
	s := &watchState{target: target{ID: "c"}, pods: map[string]pod{}, nodes: map[string]corev1.Node{"node-a": {ObjectMeta: metav1.ObjectMeta{Name: "node-a"}, Status: corev1.NodeStatus{Allocatable: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4")}}}}}
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
