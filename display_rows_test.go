package main

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestDisplayRowsHideZeroAllocatableResources(t *testing.T) {
	cpu, memory := corev1.ResourceCPU, corev1.ResourceMemory
	m := model{resourceSelected: map[corev1.ResourceName]bool{cpu: true, memory: true}, snaps: map[string]snapshot{"c": {Target: target{ID: "c"}, Capacity: qtys{cpu: resource.MustParse("1"), memory: resource.MustParse("0")}, Pods: []pod{{Requests: qtys{cpu: resource.MustParse("1"), memory: resource.MustParse("1Mi")}}}}}}
	rows := m.displayRows()
	if len(rows) != 1 || rows[0].resource != cpu {
		t.Fatalf("rows=%#v", rows)
	}
}

func TestDisplayRowsIncludesEverySelectedResource(t *testing.T) {
	cpu, memory := corev1.ResourceCPU, corev1.ResourceMemory
	m := model{resourceSelected: map[corev1.ResourceName]bool{cpu: true, memory: true}, snaps: map[string]snapshot{"c": {Target: target{ID: "c", Context: "c"}, Pods: []pod{{Requests: qtys{cpu: resource.MustParse("1"), memory: resource.MustParse("1Mi")}}}}}}
	if got := len(m.displayRows()); got != 2 {
		t.Fatalf("display rows=%d, want 2", got)
	}
}
