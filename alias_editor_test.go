package main

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	corev1 "k8s.io/api/core/v1"
)

func TestEditorAliasRuleParsesDirectConditions(t *testing.T) {
	m := model{aliasFields: [5]string{"atlas", "example.com/gpu", "model=910C, zone=a", "pool=fast", "2"}}
	rule, err := m.editorAliasRule()
	if err != nil {
		t.Fatal(err)
	}
	if rule.Resource != "example.com/gpu" || rule.Alias != "atlas" || rule.Unit != 2 || rule.Labels["model"] != "910C" || rule.Annotations["pool"] != "fast" {
		t.Fatalf("unexpected rule: %#v", rule)
	}
}

func TestParseConditionsAllowsEmptyLabelValue(t *testing.T) {
	conditions, err := parseConditions("node-role.kubernetes.io/npu-a3-560t=")
	if err != nil || conditions["node-role.kubernetes.io/npu-a3-560t"] != "" {
		t.Fatalf("conditions=%#v err=%v", conditions, err)
	}
}

func TestParseConditionsRejectsMalformedPair(t *testing.T) {
	if _, err := parseConditions("model"); err == nil {
		t.Fatal("expected malformed condition error")
	}
}

func TestAddingAliasSelectsResourceBeforeEditing(t *testing.T) {
	m := model{aliasCreating: true, resource: corev1.ResourceMemory}
	updated, _ := m.updateAlias("a")
	m = updated.(model)
	if !m.aliasSelectingResource || m.aliasEditing {
		t.Fatalf("expected resource selection: %#v", m)
	}
	updated, _ = m.updateAlias("enter")
	m = updated.(model)
	if m.aliasSelectingResource || m.aliasPicking != 1 || m.aliasEditing || m.aliasFields[1] != string(corev1.ResourceCPU) {
		t.Fatalf("expected selected resource labels: %#v", m)
	}
}

func TestAliasMetadataPickerPages(t *testing.T) {
	m := model{height: 16, aliasPicking: 1, aliasConditions: map[string]bool{}, snaps: map[string]snapshot{"c": {Nodes: []nodeInfo{{Labels: map[string]string{"a": "1", "b": "1", "c": "1", "d": "1"}}}}}}
	updated, _ := m.updateAlias("pgdown")
	if got := updated.(model).aliasCursor; got != 3 {
		t.Fatalf("cursor=%d, want 3", got)
	}
}

func TestNewAliasSelectsObservedLabels(t *testing.T) {
	m := model{aliasPicking: 1, aliasPickerReturnField: -1, aliasConditions: map[string]bool{}, snaps: map[string]snapshot{"c": {Nodes: []nodeInfo{{Labels: map[string]string{"model": "910C"}}}}}}
	updated, _ := m.updateAlias(" ")
	m = updated.(model)
	updated, _ = m.updateAlias("enter")
	m = updated.(model)
	if m.aliasFields[2] != "model=910C" || m.aliasPicking != 2 {
		t.Fatalf("expected selected labels: %#v", m)
	}
}

func TestEditingAliasReopensSelectedLabels(t *testing.T) {
	m := model{aliasEditing: true, aliasField: 2, aliasFields: [5]string{"atlas", "gpu", "model=910C", "", "1"}, snaps: map[string]snapshot{"c": {Nodes: []nodeInfo{{Labels: map[string]string{"model": "910C"}}}}}}
	updated, _ := m.updateAlias("enter")
	m = updated.(model)
	if m.aliasPicking != 1 || !m.aliasConditions["model\x00910C"] {
		t.Fatalf("expected preselected label picker: %#v", m)
	}
	updated, _ = m.updateAlias("enter")
	m = updated.(model)
	if !m.aliasEditing || m.aliasPicking != 0 || m.aliasField != 2 || m.aliasFields[2] != "model=910C" {
		t.Fatalf("expected return to label field: %#v", m)
	}
}

func TestCtrlCQuitsFromAliasEditor(t *testing.T) {
	cancelled := false
	m := model{aliasCreating: true, aliasEditing: true, cancelWatch: func() { cancelled = true }, cfgPath: filepath.Join(t.TempDir(), "config.json"), resourceSelected: map[corev1.ResourceName]bool{}}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !cancelled {
		t.Fatal("watch was not cancelled")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("ctrl+c did not quit")
	}
}

func TestEnterMovesToNextAliasField(t *testing.T) {
	m := model{aliasCreating: true, aliasEditing: true, aliasField: 4}
	updated, _ := m.updateAlias("enter")
	if got := updated.(model); got.aliasField != 0 || !got.aliasEditing {
		t.Fatalf("enter should only move fields: %#v", got)
	}
}

func TestSavingAliasSelectsItForDisplay(t *testing.T) {
	raw, alias := corev1.ResourceName("example.com/gpu"), corev1.ResourceName("atlas")
	m := model{
		cfgPath:          filepath.Join(t.TempDir(), "config.json"),
		resource:         raw,
		resourceSelected: map[corev1.ResourceName]bool{raw: true},
		aliasCreating:    true,
		aliasEditing:     true,
		aliasEditIndex:   -1,
		aliasField:       4,
		aliasFields:      [5]string{"atlas", string(raw), "model=910C", "", "1"},
	}
	updated, _ := m.updateAlias("ctrl+s")
	got := updated.(model)
	if got.resource != alias || !got.resourceSelected[alias] {
		t.Fatalf("alias resource not selected: resource=%q selected=%v", got.resource, got.resourceSelected)
	}
}
