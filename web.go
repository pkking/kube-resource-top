package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// webApp accumulates watch snapshots and serves the read-only dashboard
// (ADR 0034). The browser polls /api/state.json as a view refresh in ADR
// 0027's sense; cluster state still comes exclusively from the watch (ADR
// 0001). Aggregates reuse the model's accounting path so the page and the
// TUI can never disagree.
type webApp struct {
	mu               sync.RWMutex
	snaps            map[string]snapshot
	targets          []target
	aliases          []aliasRule
	resourceSelected map[corev1.ResourceName]bool
	selected         map[string]bool
	rec              *webRecorder
	gh               *ghClient
}

// runWeb replaces the TUI with the dashboard server. Discovery, config, and
// target selection follow the same rules as main(); there is no interactive
// selector, so the persisted selection is used as saved and unreachable
// clusters surface through their snapshot errors.
func runWeb(addr, dir, configPath string, rec *webRecorder, ghToken string) error {
	targets, _, err := discover(dir)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return fmt.Errorf("no kubeconfig contexts found")
	}
	cfg := loadConfig(configPath)
	if err := validateAliases(cfg.Aliases); err != nil {
		return err
	}
	selected, _ := selectTargets(cfg, targets)
	resources := map[corev1.ResourceName]bool{}
	for _, r := range cfg.Resources {
		resources[corev1.ResourceName(r)] = true
	}
	if len(resources) == 0 {
		resources[corev1.ResourceCPU] = true
		resources[corev1.ResourceMemory] = true
	}
	for _, rule := range cfg.Aliases { // mirrors model.selectAliases
		resources[corev1.ResourceName(rule.Alias)] = true
	}
	app := &webApp{snaps: map[string]snapshot{}, targets: targets, aliases: cfg.Aliases, resourceSelected: resources, selected: selected, rec: rec}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	updates := make(chan resultMsg, 1)
	for _, t := range targets {
		if selected[t.ID] {
			go watchTarget(ctx, t, cfg.Aliases, updates)
		}
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-updates:
				app.mu.Lock()
				app.snaps[msg.s.Target.ID] = msg.s
				app.mu.Unlock()
			}
		}
	}()
	if rec != nil {
		go rec.run(ctx, app)
	}
	if ghToken != "" {
		if gh, err := newGHClient(ghToken); err == nil {
			app.gh = gh
			go gh.start(ctx, app)
			fmt.Printf("PR 映射已启用（GitHub token 已加载）\n")
		} else {
			fmt.Fprintf(os.Stderr, "PR 映射未启用：%v\n", err)
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/state.json", app.handleState)
	mux.HandleFunc("/api/history.json", app.handleHistory)
	mux.HandleFunc("/", app.handlePage)
	srv := &http.Server{Addr: addr, Handler: mux}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	host := addr
	if h, p, err := net.SplitHostPort(addr); err == nil && (h == "" || h == "0.0.0.0" || h == "::") {
		host = "localhost:" + p
	}
	fmt.Printf("kube-resource-top 算力看板: http://%s\n", host)
	select {
	case err := <-errc:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
		return nil
	}
}

func (a *webApp) handlePage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; base-uri 'none'; form-action 'none'")
	fmt.Fprint(w, webPage)
}

func (a *webApp) handleState(w http.ResponseWriter, r *http.Request) {
	st := a.state()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_ = json.NewEncoder(w).Encode(st)
}

// handleHistory serves downsampled occupancy history for one resource and
// one cluster (or all) from the JSONL days (ADR 0040).
func (a *webApp) handleHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if a.rec == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "历史记录未启用：请用 --web-record <目录> 启动 kube-resource-top"})
		return
	}
	q := r.URL.Query()
	resource := q.Get("res")
	cluster := q.Get("cluster")
	if cluster == "" {
		cluster = "all"
	}
	hours, err := strconv.ParseFloat(q.Get("hours"), 64)
	if err != nil || hours < 1 {
		hours = 24
	}
	if hours > 24*31 {
		hours = 24 * 31
	}
	now := time.Now()
	points, err := readHistory(a.rec.dir, resource, cluster, now.Add(-time.Duration(hours*float64(time.Hour))), now, 288)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(struct {
		Resource string      `json:"resource"`
		Cluster  string      `json:"cluster"`
		Points   []histPoint `json:"points"`
	}{resource, cluster, points})
}

// webClusterMeta is the cluster list at the top of the state.
type webClusterMeta struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Server string `json:"server,omitempty"`
	Err    string `json:"err,omitempty"`
}

// webNode is one machine with positive allocatable capacity for a resource.
type webNode struct {
	Name        string  `json:"name"`
	IP          string  `json:"ip,omitempty"`
	Allocatable float64 `json:"allocatable"`
	Running     float64 `json:"running"`
	Free        float64 `json:"free"`
}

// webQueueGroup is one workload's share of a cluster's queue for a resource.
type webQueueGroup struct {
	Position    int     `json:"position"`
	Namespace   string  `json:"namespace"`
	Workload    string  `json:"workload"`
	Project     string  `json:"project"`
	Pods        int     `json:"pods"`
	Demand      float64 `json:"demand"`
	LargestPod  float64 `json:"largestPod"`
	OldestSince int64   `json:"oldestSince"` // unix ms; 0 = unknown
}

// webProject is one project's demand for a resource (ADR 0037). Name "" is
// the unattributed remainder.
type webProject struct {
	Name    string  `json:"name"`
	Running float64 `json:"running"`
	Pending float64 `json:"pending"`
}

// webClusterRes is one cluster's figures for one resource.
type webClusterRes struct {
	ClusterID   string          `json:"cluster"`
	Total       float64         `json:"total"`
	Allocatable float64         `json:"allocatable"`
	Running     float64         `json:"running"`
	Pending     float64         `json:"pending"`
	Ratio       float64         `json:"ratio"` // request/allocatable; -1 unknown
	Free        float64         `json:"free"`
	FreeNodes   int             `json:"freeNodes"`
	MaxNodeFree float64         `json:"maxNodeFree"`
	Absent      bool            `json:"absent"`
	Nodes       []webNode       `json:"nodes"`
	Queue       []webQueueGroup `json:"queue"`
	Projects    []webProject    `json:"projects"`
	Verdict     string          `json:"verdict"` // full|fragmented|blocked|absent|empty
	Note        string          `json:"note"`
}

// webTask is one workload's running demand for a resource (ADR 0042),
// unaggregated — the page owns the per-zone display policy. PR fields
// carry the GitHub mapping when a token is configured (ADR 0045).
type webTask struct {
	Cluster  string  `json:"cluster"` // target ID; the page maps it to a name
	Workload string  `json:"workload"`
	Project  string  `json:"project"`
	Run      float64 `json:"run"`
	PR       int     `json:"pr,omitempty"`
	RunURL   string  `json:"runUrl,omitempty"`
	JobURL   string  `json:"jobUrl,omitempty"`
	Branch   string  `json:"branch,omitempty"`
	Workflow string  `json:"workflow,omitempty"`
}

// webResource is the cross-cluster aggregate for one resource type.
type webResource struct {
	Name             string          `json:"name"`
	Unit             string          `json:"unit"`
	Total            float64         `json:"total"`
	Allocatable      float64         `json:"allocatable"`
	Running          float64         `json:"running"`
	Pending          float64         `json:"pending"`
	Ratio            float64         `json:"ratio"`    // 需求 (running+pending)/allocatable; -1 unknown (ADR 0028)
	RunningRatio     float64         `json:"runRatio"` // 占用 running/allocatable; -1 unknown (ADR 0035)
	CapOK            bool            `json:"capOK"`
	PendingWorkloads int             `json:"pendingWorkloads"`
	Projects         []webProject    `json:"projects"`
	Tasks            []webTask       `json:"tasks"`
	Clusters         []webClusterRes `json:"clusters"`
}

type webState struct {
	At        string           `json:"at"`
	NowMs     int64            `json:"nowMs"`
	Live      int              `json:"live"`
	Total     int              `json:"total"`
	Clusters  []webClusterMeta `json:"clusters"`
	Resources []webResource    `json:"resources"`
}

// state shapes the whole dashboard payload from the current snapshots. It
// runs under the read lock; watchers only replace whole snapshot values.
func (a *webApp) state() webState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	m := model{snaps: a.snaps, aliases: a.aliases, resourceSelected: a.resourceSelected, selected: a.selected}
	now := time.Now()
	st := webState{At: now.Format(time.RFC3339), NowMs: now.UnixMilli(), Clusters: []webClusterMeta{}, Resources: []webResource{}}
	// The cluster list comes from the selected targets, not the snapshots: a
	// cluster whose watch never delivers a snapshot must stay visible as
	// "not connected" instead of silently vanishing from the board.
	for _, t := range a.targets {
		if !a.selected[t.ID] {
			continue
		}
		st.Total++
		c := webClusterMeta{ID: t.ID, Name: webTargetName(t), Server: t.Server}
		if s, ok := a.snaps[t.ID]; ok && s.Err != "" {
			c.Err = s.Err
		} else if ok {
			st.Live++
		} else {
			c.Err = "尚未收到快照：集群不可达或仍在同步"
		}
		st.Clusters = append(st.Clusters, c)
	}
	sort.Slice(st.Clusters, func(i, j int) bool { return st.Clusters[i].Name < st.Clusters[j].Name })
	for _, r := range m.accountedResources() {
		rm := m
		rm.scope = nil
		rm.resource = r
		rm.sortBy = 4 // name order, like the TUI aggregate context rows
		rows := rm.rows()
		res := webResource{Name: string(r), Unit: webUnit(r), CapOK: true, Ratio: -1, RunningRatio: -1, Clusters: []webClusterRes{}, Projects: []webProject{}, Tasks: []webTask{}}
		have := map[string]bool{}
		projects := map[string]*webProject{}
		tasks := map[string]*webTask{}
		for _, row := range rows {
			have[row.key] = true
			s := a.snaps[row.key]
			cr := newWebClusterRes(s, r, row, false)
			res.Clusters = append(res.Clusters, cr)
			res.Total += cr.Total
			res.Allocatable += cr.Allocatable
			res.Running += cr.Running
			res.Pending += cr.Pending
			res.PendingWorkloads += len(cr.Queue)
			if _, ok := s.Capacity[r]; !ok {
				res.CapOK = false
			}
			// Project split mirrors the row sums exactly (offloaded pods
			// excluded, label-derived running demand included — ADR 0037).
			webSumProjects(s, r, projects)
			// Running tasks at workload granularity for the strip blocks
			// (ADR 0042), same pod filter as the sums.
			webSumTasks(s, r, row.key, tasks)
		}
		// Clusters with zero allocatable capacity for the resource are kept
		// out of the aggregates (ADR 0013), but queued demand on them gets
		// an explicit "absent" row: the queue user's question is exactly
		// "why isn't my task running" (ADR 0034 narrows ADR 0013).
		for id, s := range a.snaps {
			if s.Err != "" || have[id] {
				continue
			}
			if webQty(r, snapshotDemand(s, r, true)) > 0 {
				cr := newWebClusterRes(s, r, row{key: id}, true)
				res.Clusters = append(res.Clusters, cr)
				res.PendingWorkloads += len(cr.Queue)
			}
		}
		for _, pr := range projects {
			res.Projects = append(res.Projects, *pr)
		}
		for _, t := range tasks {
			res.Tasks = append(res.Tasks, *t)
		}
		// Task order: demand first, then names — deterministic files and
		// tests; the page regroups into project zones anyway.
		sort.Slice(res.Tasks, func(i, j int) bool {
			a, b := res.Tasks[i], res.Tasks[j]
			if a.Run != b.Run {
				return a.Run > b.Run
			}
			return a.Cluster+"\x00"+a.Workload < b.Cluster+"\x00"+b.Workload
		})
		if res.Tasks == nil {
			res.Tasks = []webTask{}
		}
		// Attach the GitHub PR mapping by runner name (ADR 0045).
		if a.gh != nil {
			for i := range res.Tasks {
				if info, ok := a.gh.lookup(res.Tasks[i].Workload); ok {
					res.Tasks[i].PR = info.PR
					res.Tasks[i].RunURL = info.RunURL
					res.Tasks[i].JobURL = info.JobURL
					res.Tasks[i].Branch = info.Branch
					res.Tasks[i].Workflow = info.Workflow
				}
			}
		}
		// Demand first, then name; the unattributed "" sorts where its
		// demand puts it, exactly like any project.
		sort.Slice(res.Projects, func(i, j int) bool {
			a, b := res.Projects[i], res.Projects[j]
			ai, aj := a.Running+a.Pending, b.Running+b.Pending
			if ai != aj {
				return ai > aj
			}
			return a.Name < b.Name
		})
		if res.CapOK && res.Allocatable > 0 {
			res.Ratio = (res.Running + res.Pending) / res.Allocatable
			res.RunningRatio = res.Running / res.Allocatable
		} else {
			res.CapOK = false
			res.Ratio = -1
			res.RunningRatio = -1
		}
		st.Resources = append(st.Resources, res)
	}
	return st
}

func webTargetName(t target) string {
	return filepath.Base(t.Path) + " :: " + display(t.Context)
}

// newWebClusterRes builds one cluster's figures. row carries the level-0 sums
// from rows() (capacity, total, running, pending with offloaded pods
// excluded, ADRs 0031/0033); the absent path derives pending straight from
// the snapshot because rows() never returns zero-allocatable clusters.
func newWebClusterRes(s snapshot, r corev1.ResourceName, row row, absent bool) webClusterRes {
	cr := webClusterRes{ClusterID: row.key}
	cr.Total = webQty(r, row.t)
	cr.Allocatable = webQty(r, row.c)
	cr.Running = webQty(r, row.running)
	if absent {
		cr.Pending = webQty(r, snapshotDemand(s, r, true))
	} else {
		cr.Pending = webQty(r, row.pending)
	}
	if _, ok := s.Capacity[r]; ok && cr.Allocatable > 0 {
		cr.Ratio = (cr.Running + cr.Pending) / cr.Allocatable
	} else {
		cr.Ratio = -1
	}
	cr.Nodes = webNodes(s, r)
	cr.Free = cr.Allocatable - cr.Running
	for _, n := range cr.Nodes {
		if n.Free > 0 {
			cr.FreeNodes++
			if n.Free > cr.MaxNodeFree {
				cr.MaxNodeFree = n.Free
			}
		}
	}
	cr.Queue = webQueue(s, r)
	cr.Projects = webProjects(s, r)
	cr.Absent = absent
	cr.Verdict, cr.Note = webVerdict(r, cr)
	return cr
}

// webProjects aggregates one snapshot's per-project demand for a resource
// (offloaded pods excluded), sorted by demand then name.
func webProjects(s snapshot, r corev1.ResourceName) []webProject {
	into := map[string]*webProject{}
	webSumProjects(s, r, into)
	var out []webProject
	for _, p := range into {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		ai, aj := a.Running+a.Pending, b.Running+b.Pending
		if ai != aj {
			return ai > aj
		}
		return a.Name < b.Name
	})
	if out == nil {
		out = []webProject{} // never null (the page indexes .length)
	}
	return out
}

// snapshotDemand sums pod requests for r over pending or running pods,
// excluding offloaded (remote) work exactly like rows().
func snapshotDemand(s snapshot, r corev1.ResourceName, pending bool) resource.Quantity {
	var out resource.Quantity
	for _, p := range s.Pods {
		if p.Offloaded {
			continue
		}
		if pending != (p.Phase == corev1.PodPending) {
			continue
		}
		out.Add(p.Requests[r])
	}
	return out
}

// webNodes lists each machine's allocatable, running, and free amount for
// one resource, mirroring the node balance view: only nodes with positive
// allocatable appear, and running demand is attributed by pod placement.
func webNodes(s snapshot, r corev1.ResourceName) []webNode {
	running := map[string]float64{}
	for _, p := range s.Pods {
		if p.Offloaded || p.Phase != corev1.PodRunning || p.NodeName == "" {
			continue
		}
		running[p.NodeName] += webQty(r, p.Requests[r])
	}
	var out []webNode
	for _, n := range s.Nodes {
		alloc := webQty(r, n.Capacity[r])
		if alloc <= 0 {
			continue
		}
		run := running[n.Name]
		out = append(out, webNode{Name: n.Name, IP: n.IP, Allocatable: alloc, Running: run, Free: alloc - run})
	}
	if out == nil {
		out = []webNode{} // marshals as [] instead of null; the page indexes .length
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// webSumProjects accumulates per-project running and pending demand for a
// resource from one snapshot, using the same pod filter as the row sums
// (offloaded work excluded, ADRs 0031/0033).
func webSumProjects(s snapshot, r corev1.ResourceName, into map[string]*webProject) {
	for _, p := range s.Pods {
		if p.Offloaded {
			continue
		}
		q := webQty(r, p.Requests[r])
		if q == 0 {
			continue
		}
		pr := into[p.Project]
		if pr == nil {
			pr = &webProject{Name: p.Project}
			into[p.Project] = pr
		}
		if p.Phase == corev1.PodPending {
			pr.Pending += q
		} else {
			pr.Running += q
		}
	}
}

// webSumTasks accumulates running demand per (workload, project) from one
// snapshot (ADR 0042), using the same pod filter as the row sums.
func webSumTasks(s snapshot, r corev1.ResourceName, clusterID string, into map[string]*webTask) {
	for _, p := range s.Pods {
		if p.Offloaded || p.Phase != corev1.PodRunning {
			continue
		}
		q := webQty(r, p.Requests[r])
		if q == 0 {
			continue
		}
		key := p.Workload + "\x00" + p.Project
		t := into[key]
		if t == nil {
			t = &webTask{Cluster: clusterID, Workload: p.Workload, Project: p.Project}
			into[key] = t
		}
		t.Run += q
	}
}

// webQueue groups a cluster's unscheduled demand for one resource by
// workload and orders groups by their earliest pod creation time — the
// honest approximation of a queue position, since Kubernetes itself may
// reorder by priority or affinity. Unknown timestamps sort last.
func webQueue(s snapshot, r corev1.ResourceName) []webQueueGroup {
	type group struct {
		namespace, workload, project string
		pods                         int
		demand, largest              float64
		oldest                       time.Time
	}
	groups := map[string]*group{}
	for _, p := range s.Pods {
		if p.Phase != corev1.PodPending || p.Offloaded {
			continue
		}
		request := p.Requests[r]
		if request.IsZero() {
			continue
		}
		key := p.Namespace + "\x00" + p.Workload
		g := groups[key]
		if g == nil {
			g = &group{namespace: p.Namespace, workload: p.Workload}
			groups[key] = g
		}
		if g.project == "" {
			g.project = p.Project // one workload, one project (ADR 0037)
		}
		g.pods++
		d := webQty(r, request)
		g.demand += d
		if d > g.largest {
			g.largest = d
		}
		if !p.CreatedAt.IsZero() && (g.oldest.IsZero() || p.CreatedAt.Before(g.oldest)) {
			g.oldest = p.CreatedAt
		}
	}
	out := make([]webQueueGroup, 0, len(groups))
	for _, g := range groups {
		var since int64
		if !g.oldest.IsZero() {
			since = g.oldest.UnixMilli()
		}
		out = append(out, webQueueGroup{Namespace: g.namespace, Workload: g.workload, Project: g.project, Pods: g.pods, Demand: g.demand, LargestPod: g.largest, OldestSince: since})
	}
	sort.Slice(out, func(i, j int) bool {
		oi, oj := out[i].OldestSince, out[j].OldestSince
		if (oi == 0) != (oj == 0) {
			return oj == 0 // unknown ages never claim the head position
		}
		if oi != oj {
			return oi < oj
		}
		return out[i].Namespace+"\x00"+out[i].Workload < out[j].Namespace+"\x00"+out[j].Workload
	})
	for i := range out {
		out[i].Position = i + 1
	}
	return out
}

// webVerdict answers "is the cluster really full" from observed evidence
// only. No ETA: the watch has no completion history, so wait time stays a
// fact the user weighs (queue depth, head age, free capacity), not a
// prediction (ADR 0034).
func webVerdict(r corev1.ResourceName, cr webClusterRes) (string, string) {
	if len(cr.Queue) == 0 {
		return "empty", "没有排队任务。"
	}
	if cr.Allocatable <= 0 {
		return "absent", "集群没有这种资源（可分配为 0），任务无法调度。"
	}
	head := cr.Queue[0]
	if cr.Free <= 0 {
		return "full", fmt.Sprintf("可分配容量已用完（运行中 %s），排队任务需等待运行中的任务释放。", webQtyText(r, cr.Running))
	}
	if cr.MaxNodeFree < head.LargestPod {
		return "fragmented", fmt.Sprintf("还有 %s 空闲，但分散在 %d 台机器上（单机最多 %s），放不下队首任务（单个 Pod 需 %s）。", webQtyText(r, cr.Free), cr.FreeNodes, webQtyText(r, cr.MaxNodeFree), webQtyText(r, head.LargestPod))
	}
	return "blocked", fmt.Sprintf("有空闲的 %s（单机最多 %s）能容纳队首任务，但它仍未被调度——可能在等待特定节点亲和性、污点或优先级。", webQtyText(r, cr.Free), webQtyText(r, cr.MaxNodeFree))
}

// webUnit picks the display noun for a resource kind.
func webUnit(r corev1.ResourceName) string {
	switch r {
	case corev1.ResourceCPU:
		return "核"
	case corev1.ResourceMemory:
		return "GiB"
	default:
		return "张"
	}
}

// webQty converts a quantity to a display number: CPU in cores, memory in
// GiB, and card-like extended resources as counted units (milli-precision
// from alias unit scaling keeps fractional values).
func webQty(r corev1.ResourceName, q resource.Quantity) float64 {
	if q.IsZero() {
		return 0
	}
	switch r {
	case corev1.ResourceCPU:
		return float64(q.MilliValue()) / 1000
	case corev1.ResourceMemory:
		return float64(q.Value()) / (1 << 30)
	default:
		return q.AsApproximateFloat64()
	}
}

func webQtyText(r corev1.ResourceName, v float64) string {
	if v != math.Trunc(v) {
		return fmt.Sprintf("%.1f %s", v, webUnit(r))
	}
	return fmt.Sprintf("%.0f %s", v, webUnit(r))
}
