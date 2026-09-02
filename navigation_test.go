package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestContextShortcutReopensSelector(t *testing.T) {
	m := model{targets: []target{{ID: "a", Probe: "available"}, {ID: "b", Probe: "available"}}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	got := updated.(model)
	if !got.choosing || got.probePending != 2 || got.targets[0].Probe != "" {
		t.Fatalf("model=%#v", got)
	}
}

func TestSearchFiltersVisibleRows(t *testing.T) {
	cpu := corev1.ResourceCPU
	m := model{height: 15, resourceSelected: map[corev1.ResourceName]bool{cpu: true}, search: "payments", snaps: map[string]snapshot{
		"a": {Target: target{ID: "a", Context: "payments"}, Pods: []pod{{Requests: qtys{cpu: resource.MustParse("1")}}}},
		"b": {Target: target{ID: "b", Context: "platform"}, Pods: []pod{{Requests: qtys{cpu: resource.MustParse("1")}}}},
	}}
	if got := len(m.filteredDisplayRows()); got != 1 {
		t.Fatalf("rows=%d, want 1", got)
	}
}
