package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// recordLine is one (cluster, resource) occupancy sample in the JSONL
// history (ADR 0040). Short JSON keys keep the daily files compact.
type recordLine struct {
	At          int64           `json:"t"` // unix seconds
	Cluster     string          `json:"c"` // cluster display name
	Resource    string          `json:"r"` // resource name
	Total       float64         `json:"T"`
	Allocatable float64         `json:"A"`
	Running     float64         `json:"R"`
	Pending     float64         `json:"P"`
	Queue       int             `json:"Q,omitempty"`
	Projects    []recordProject `json:"p,omitempty"`
}

type recordProject struct {
	N    string  `json:"n"` // "" = unattributed
	Run  float64 `json:"r"`
	Pend float64 `json:"q"`
}

// webRecorder samples the in-memory watch state to JSONL files on a timer
// (ADR 0040). It never contacts a cluster: it re-reads state the watch
// already delivered, exactly like the display tick (ADR 0027).
type webRecorder struct {
	mu       sync.Mutex
	dir      string
	interval time.Duration
	keep     int
	day      string
	file     *os.File
}

func newWebRecorder(dir string, interval time.Duration, keep int) *webRecorder {
	return &webRecorder{dir: dir, interval: interval, keep: keep}
}

func (rec *webRecorder) run(ctx context.Context, a *webApp) {
	if err := os.MkdirAll(rec.dir, 0o755); err != nil {
		return
	}
	tick := time.NewTicker(rec.interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			rec.mu.Lock()
			if rec.file != nil {
				_ = rec.file.Close()
				rec.file = nil
			}
			rec.mu.Unlock()
			return
		case now := <-tick.C:
			rec.sample(a, now)
		}
	}
}

func (rec *webRecorder) sample(a *webApp, now time.Time) {
	lines := recordLines(a.state(), now)
	if len(lines) == 0 {
		return
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if err := rec.rotateLocked(now); err != nil {
		return
	}
	if rec.file == nil {
		return
	}
	w := bufio.NewWriter(rec.file)
	for _, line := range lines {
		b, err := json.Marshal(line)
		if err != nil {
			continue
		}
		w.Write(b)
		w.WriteByte('\n')
	}
	_ = w.Flush()
}

// rotateLocked opens today's file, closing yesterday's, gzipping it and
// pruning files older than the keep window (ADR 0040).
func (rec *webRecorder) rotateLocked(now time.Time) error {
	day := now.UTC().Format("2006-01-02")
	if rec.file != nil && rec.day == day {
		return nil
	}
	if rec.file != nil {
		_ = rec.file.Close()
		rec.file = nil
	}
	if rec.day != "" {
		if err := gzipFile(filepath.Join(rec.dir, "history-"+rec.day+".jsonl")); err != nil {
			return err
		}
	}
	rec.day = day
	f, err := os.OpenFile(filepath.Join(rec.dir, "history-"+day+".jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	rec.file = f
	rec.pruneLocked(now)
	return nil
}

func (rec *webRecorder) pruneLocked(now time.Time) {
	entries, err := os.ReadDir(rec.dir)
	if err != nil {
		return
	}
	cutoff := now.AddDate(0, 0, -rec.keep).UTC().Format("2006-01-02")
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "history-") {
			continue
		}
		date := strings.TrimSuffix(strings.TrimSuffix(name, ".jsonl"), ".gz")
		date = strings.TrimPrefix(strings.TrimSuffix(date, ".jsonl"), "history-")
		if len(date) != 10 || date >= cutoff {
			continue
		}
		_ = os.Remove(filepath.Join(rec.dir, name))
	}
}

func gzipFile(path string) error {
	in, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(path+".gz", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	w := gzip.NewWriter(out)
	if _, err := bufio.NewReader(in).WriteTo(w); err != nil {
		_ = w.Close()
		_ = out.Close()
		return err
	}
	if err := w.Close(); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Remove(path)
}

// recordLines shapes one sample from the live dashboard state, pure for
// tests. Line order is deterministic: resources in their board order,
// clusters within each.
func recordLines(st webState, now time.Time) []recordLine {
	names := map[string]string{}
	for _, c := range st.Clusters {
		names[c.ID] = c.Name
	}
	var out []recordLine
	at := now.Unix()
	for _, res := range st.Resources {
		for _, c := range res.Clusters {
			line := recordLine{
				At:          at,
				Cluster:     names[c.ClusterID],
				Resource:    res.Name,
				Total:       c.Total,
				Allocatable: c.Allocatable,
				Running:     c.Running,
				Pending:     c.Pending,
				Queue:       len(c.Queue),
			}
			for _, p := range c.Projects {
				line.Projects = append(line.Projects, recordProject{N: p.Name, Run: p.Running, Pend: p.Pending})
			}
			if line.Cluster != "" {
				out = append(out, line)
			}
		}
	}
	return out
}

// histPoint is one downsampled, cluster-aggregated point of history.
type histPoint struct {
	At       int64         `json:"t"` // unix ms
	Alloc    float64       `json:"A"`
	Run      float64       `json:"R"`
	Pend     float64       `json:"P"`
	Projects []histProject `json:"p"`
}

type histProject struct {
	N    string  `json:"n"`
	Run  float64 `json:"r"`
	Pend float64 `json:"q"`
}

// readHistory loads the JSONL days covering [from, to], filters to one
// resource and one cluster (or all), keeps the last sample per (cluster,
// bucket), sums across clusters, and downsamples to at most maxPoints
// bucket-end points (ADR 0040).
func readHistory(dir, resource, cluster string, from, to time.Time, maxPoints int) ([]histPoint, error) {
	if maxPoints <= 0 {
		maxPoints = 288
	}
	bucket := int64(to.Sub(from).Seconds()/float64(maxPoints)) + 1
	last := map[bucketKey]recordLine{}
	// Iterate UTC calendar days: from.Truncate(24h) lands on UTC midnight,
	// so a range crossing midnight opens both days' files (bug: stepping
	// from+24h skipped the current day between 00:00 and 01:00 UTC).
	for day := from.Truncate(24 * time.Hour).UTC(); !day.After(to); day = day.AddDate(0, 0, 1) {
		name := "history-" + day.Format("2006-01-02") + ".jsonl"
		path := filepath.Join(dir, name)
		f, err := os.Open(path)
		if err != nil {
			gz, gzErr := os.Open(path + ".gz")
			if gzErr != nil {
				continue
			}
			zr, zErr := gzip.NewReader(gz)
			if zErr != nil {
				gz.Close()
				continue
			}
			err = scanHistory(zr, resource, cluster, from, to, bucket, last)
			gz.Close()
			if err != nil {
				return nil, err
			}
			continue
		}
		err = scanHistory(f, resource, cluster, from, to, bucket, last)
		f.Close()
		if err != nil {
			return nil, err
		}
	}
	buckets := map[int64]map[string]recordLine{}
	for k, line := range last {
		if buckets[k.bucket] == nil {
			buckets[k.bucket] = map[string]recordLine{}
		}
		buckets[k.bucket][k.cluster] = line
	}
	idxs := make([]int64, 0, len(buckets))
	for i := range buckets {
		idxs = append(idxs, i)
	}
	sort.Slice(idxs, func(a, b int) bool { return idxs[a] < idxs[b] })
	var out []histPoint
	for _, idx := range idxs {
		pt := histPoint{}
		proj := map[string]*histProject{}
		for _, line := range buckets[idx] {
			pt.Alloc += line.Allocatable
			pt.Run += line.Running
			pt.Pend += line.Pending
			for _, p := range line.Projects {
				e := proj[p.N]
				if e == nil {
					e = &histProject{N: p.N}
					proj[p.N] = e
				}
				e.Run += p.Run
				e.Pend += p.Pend
			}
		}
		for _, p := range proj {
			pt.Projects = append(pt.Projects, *p)
		}
		// Name-stable order so stacked areas do not jitter between points.
		sort.Slice(pt.Projects, func(a, b int) bool { return pt.Projects[a].N < pt.Projects[b].N })
		if pt.Projects == nil {
			pt.Projects = []histProject{}
		}
		pt.At = (idx*bucket + bucket/2) * 1000 // bucket midpoint, ms
		out = append(out, pt)
	}
	return out, nil
}

type bucketKey struct {
	bucket  int64
	cluster string
}

func scanHistory(r interface{ Read([]byte) (int, error) }, resource, cluster string, from, to time.Time, bucket int64, last map[bucketKey]recordLine) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lo, hi := from.Unix(), to.Unix()
	for sc.Scan() {
		var line recordLine
		if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
			continue // a torn line never breaks history
		}
		if resource != "" && line.Resource != resource {
			continue
		}
		if cluster != "" && cluster != "all" && line.Cluster != cluster {
			continue
		}
		if line.At < lo || line.At > hi {
			continue
		}
		last[bucketKey{line.At / bucket, line.Cluster}] = line
	}
	return sc.Err()
}
