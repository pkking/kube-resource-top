package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelectTargetsAddsNewKubeconfigWithoutReselectingKnownTarget(t *testing.T) {
	targets := []target{{ID: "selected"}, {ID: "deselected"}, {ID: "new"}}
	selected, first := selectTargets(config{Targets: []string{"selected"}, KnownTargets: []string{"selected", "deselected"}}, targets)
	if first || !selected["selected"] || selected["deselected"] || !selected["new"] {
		t.Fatalf("selected=%v first=%v", selected, first)
	}
}

func TestRestartDiscoversNewKubeconfig(t *testing.T) {
	dir := t.TempDir()
	writeConfig := func(name, context string) {
		t.Helper()
		content := "apiVersion: v1\nkind: Config\nclusters:\n- name: c\n  cluster:\n    server: https://example.invalid\ncontexts:\n- name: " + context + "\n  context:\n    cluster: c\n    user: u\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeConfig("first.yaml", "first")
	before, _, err := discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeConfig("new.yaml", "new")
	after, _, err := discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	selected, _ := selectTargets(config{Targets: []string{before[0].ID}, KnownTargets: []string{before[0].ID}}, after)
	if len(after) != 2 || len(selected) != 2 {
		t.Fatalf("targets=%d selected=%v", len(after), selected)
	}
}
