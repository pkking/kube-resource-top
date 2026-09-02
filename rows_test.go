package main

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestClusterCapacityIsNotMultipliedByPods(t *testing.T) {
	cpu := corev1.ResourceCPU
	m := model{resource: cpu, snaps: map[string]snapshot{"c": {Target: target{ID: "c", Context: "c"}, Capacity: qtys{cpu: resource.MustParse("10")}, Pods: []pod{{Requests: qtys{cpu: resource.MustParse("1")}}, {Requests: qtys{cpu: resource.MustParse("1")}}}}}}
	rows := m.rows()
	if len(rows) != 1 {
		t.Fatalf("rows=%d", len(rows))
	}
	capacity := rows[0].c
	if got := capacity.Value(); got != 10 {
		t.Fatalf("capacity=%d, want 10", got)
	}
}
