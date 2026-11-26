package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecordLinesFromState(t *testing.T) {
	st := webState{
		Clusters: []webClusterMeta{{ID: "a", Name: "cn1.yaml :: cn1"}},
		Resources: []webResource{{
			Name: "910C800T", Unit: "张", Total: 184, Allocatable: 184, Running: 96, Pending: 64,
			Clusters: []webClusterRes{{
				ClusterID: "a", Total: 184, Allocatable: 184, Running: 96, Pending: 64,
				Queue:    []webQueueGroup{{Position: 1}},
				Projects: []webProject{{Name: "sgl-project/sglang", Running: 14}, {Name: "", Running: 82, Pending: 64}},
			}},
		}},
	}
	now := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	lines := recordLines(st, now)
	if len(lines) != 1 {
		t.Fatalf("want 1 line, got %d", len(lines))
	}
	l := lines[0]
	if l.At != now.Unix() || l.Cluster != "cn1.yaml :: cn1" || l.Resource != "910C800T" {
		t.Fatalf("identity = %+v", l)
	}
	if l.Running != 96 || l.Pending != 64 || l.Allocatable != 184 || l.Queue != 1 || len(l.Projects) != 2 {
		t.Errorf("figures = %+v", l)
	}
	if l.Projects[0].N != "sgl-project/sglang" || l.Projects[0].Run != 14 {
		t.Errorf("projects = %+v", l.Projects)
	}
}

func TestRecorderRotationAndRetention(t *testing.T) {
	dir := t.TempDir()
	rec := newWebRecorder(dir, time.Minute, 2)
	if err := rec.rotateLocked(time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if _, err := rec.file.WriteString("{\"t\":0}\n"); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(dir, "history-2000-01-01.jsonl")
	if err := os.WriteFile(old, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := rec.rotateLocked(time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "history-2026-11-24.jsonl.gz")); err != nil {
		t.Errorf("previous day must be gzipped: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "history-2026-11-24.jsonl")); !os.IsNotExist(err) {
		t.Errorf("raw previous day must be removed")
	}
	if _, err := os.Stat(filepath.Join(dir, "history-2026-11-26.jsonl")); err != nil {
		t.Errorf("today's file must exist: %v", err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("files older than the keep window must be pruned")
	}
}

func writeRec(t *testing.T, dir string, at int64, cluster, resource string, run, pend float64) {
	t.Helper()
	day := time.Unix(at, 0).UTC().Format("2006-01-02")
	f, err := os.OpenFile(filepath.Join(dir, "history-"+day+".jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	line := recordLine{At: at, Cluster: cluster, Resource: resource, Running: run, Pending: pend, Projects: []recordProject{{N: "p1", Run: run, Pend: pend}}}
	if err := json.NewEncoder(f).Encode(line); err != nil {
		t.Fatal(err)
	}
}

func TestReadHistoryBucketsAndAggregation(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	base := now.Add(-50 * time.Minute).Unix()
	// Same bucket for c1: the later sample wins (bucket ~13s).
	writeRec(t, dir, base, "c1", "R", 10, 0)
	writeRec(t, dir, base+5, "c1", "R", 12, 0)
	writeRec(t, dir, base, "c2", "R", 4, 2)
	// A later bucket for c2, and an unrelated resource that must filter out.
	writeRec(t, dir, base+100, "c2", "R", 6, 0)
	writeRec(t, dir, base+100, "c2", "other", 99, 99)

	points, err := readHistory(dir, "R", "all", now.Add(-time.Hour), now, 288)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 2 {
		t.Fatalf("want 2 bucket points, got %d: %+v", len(points), points)
	}
	if points[0].Run != 16 || points[0].Pend != 2 || points[0].Alloc != 0 {
		t.Errorf("point 0 = %+v, want run 16 pend 2 (c1 last-wins 12 + c2 4)", points[0])
	}
	if len(points[0].Projects) != 1 || points[0].Projects[0].N != "p1" || points[0].Projects[0].Run != 16 {
		t.Errorf("point 0 projects = %+v", points[0].Projects)
	}
	if points[1].Run != 6 || points[1].Pend != 0 {
		t.Errorf("point 1 = %+v, want c2 only run 6", points[1])
	}
	if points[0].At > points[1].At {
		t.Errorf("points must be time-ordered")
	}
	// Cluster filter narrows to c2's own series.
	c2, err := readHistory(dir, "R", "c2", now.Add(-time.Hour), now, 288)
	if err != nil {
		t.Fatal(err)
	}
	if len(c2) != 2 || c2[0].Run != 4 || c2[0].Pend != 2 || c2[1].Run != 6 {
		t.Errorf("cluster filter = %+v", c2)
	}
}
