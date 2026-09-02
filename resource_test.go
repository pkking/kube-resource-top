package main

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func resourceModel() model {
	return model{resource: corev1.ResourceCPU, resourceSelected: map[corev1.ResourceName]bool{corev1.ResourceCPU: true, corev1.ResourceMemory: true}, snaps: map[string]snapshot{"c": {Pods: []pod{{Requests: qtys{corev1.ResourceName("example.com/gpu"): resource.MustParse("1")}}}}}}
}
func TestNextResourceIncludesDiscoveredKeys(t *testing.T) {
	m := resourceModel()
	if got := m.nextResource(); got != corev1.ResourceName("example.com/gpu") {
		t.Fatalf("got %q", got)
	}
}
func TestSelectedResourcesSupportsMultipleKeys(t *testing.T) {
	m := resourceModel()
	if got := len(m.selectedResources()); got != 2 {
		t.Fatalf("selected=%d, want 2", got)
	}
}
