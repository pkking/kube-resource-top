package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type config struct {
	Targets      []string    `json:"targets,omitempty"`
	KnownTargets []string    `json:"knownTargets,omitempty"`
	Resources    []string    `json:"resources,omitempty"`
	Aliases      []aliasRule `json:"aliases,omitempty"`
	Interval     int         `json:"interval,omitempty"`
}
type aliasRule struct {
	Alias       string            `json:"alias"`
	Resource    string            `json:"resource"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Unit        int64             `json:"unit,omitempty"`
}
type target struct{ ID, Path, Context, Server, Probe string }
type snapshot struct {
	Target   target
	Pods     []pod
	Nodes    []nodeInfo
	Capacity qtys
	Err      string
	At       time.Time
	Metrics  string
	Note     string
}
type nodeInfo struct {
	Name        string
	IP          string
	Labels      map[string]string
	Annotations map[string]string
	Capacity    qtys
}
type pod struct {
	Namespace, Name, Workload, UID, NodeName string
	Phase                                    corev1.PodPhase
	Offloaded                                bool
	OwnerRefs                                []metav1.OwnerReference
	Requests, Limits, Usage                  qtys
	Annotations                              map[string]string
}
type qtys map[corev1.ResourceName]resource.Quantity

type resultMsg struct{ s snapshot }
type probeMsg struct {
	id  string
	err error
}
type refreshMsg struct{}

type model struct {
	dir, cfgPath                                        string
	resourceSelected                                    map[corev1.ResourceName]bool
	resourcePicking                                     bool
	resourceCursor                                      int
	width, height                                       int
	search                                              string
	searching                                           bool
	viewMode, nodeScope                                 int
	nodeBalance                                         bool
	cfg                                                 config
	targets                                             []target
	selected                                            map[string]bool
	snaps                                               map[string]snapshot
	choosing                                            bool
	cursor                                              int
	scope                                               []string
	resource                                            corev1.ResourceName
	sortBy                                              int
	reverse                                             bool
	refreshing                                          bool
	probePending                                        int
	watchCtx                                            context.Context
	cancelWatch                                         context.CancelFunc
	updates                                             chan resultMsg
	aliases                                             []aliasRule
	aliasCreating, aliasEditing, aliasSelectingResource bool
	aliasPicking, aliasCursor, aliasField               int
	aliasPickerReturnField                              int
	aliasEditIndex                                      int
	aliasFields                                         [5]string
	aliasConditions                                     map[string]bool
	aliasError                                          string
}

var yellow = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
var red = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
var gray = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
var selectedStyle = lipgloss.NewStyle().Background(lipgloss.Color("24"))

const (
	nodeScopeDrill = iota
	nodeScopeAll
	nodeScopeContexts
	nodeScopeResources
)

func main() {
	dir := flag.String("kubeconfig-dir", "", "directory containing kubeconfig files (required)")
	configPath := flag.String("config", defaultConfigPath(), "selection config path")
	testOnly := flag.Bool("t", false, "test every kubeconfig context and report unavailable entries")
	testTimeout := flag.Duration("test-timeout", 3*time.Second, "per-context timeout for -t")
	testConcurrency := flag.Int("test-concurrency", 8, "parallel context checks for -t")
	flag.Parse()
	if *dir == "" {
		fmt.Fprintln(os.Stderr, "--kubeconfig-dir is required")
		os.Exit(2)
	}
	if *testOnly {
		os.Exit(testKubeconfigs(*dir, *testTimeout, *testConcurrency))
	}
	targets, bad, err := discover(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(targets) == 0 {
		fmt.Fprintln(os.Stderr, "no kubeconfig contexts found")
		os.Exit(1)
	}
	cfg := loadConfig(*configPath)
	if err := validateAliases(cfg.Aliases); err != nil {
		fmt.Fprintln(os.Stderr, "invalid aliases:", err)
		os.Exit(2)
	}
	if cfg.Interval == 0 {
		cfg.Interval = 5
	}
	selected, first := selectTargets(cfg, targets)
	watchCtx, cancelWatch := context.WithCancel(context.Background())
	resources := map[corev1.ResourceName]bool{}
	for _, r := range cfg.Resources {
		resources[corev1.ResourceName(r)] = true
	}
	if len(resources) == 0 {
		resources[corev1.ResourceCPU] = true
		resources[corev1.ResourceMemory] = true
	}
	m := model{dir: *dir, cfgPath: *configPath, cfg: cfg, targets: targets, selected: selected, resourceSelected: resources, snaps: map[string]snapshot{}, choosing: first, watchCtx: watchCtx, cancelWatch: cancelWatch, updates: make(chan resultMsg, 1), aliases: cfg.Aliases, width: 120, height: 30, sortBy: 4}
	if first {
		m.probePending = len(targets)
	}
	if len(cfg.Resources) > 0 {
		m.resource = corev1.ResourceName(cfg.Resources[0])
	} else {
		m.resource = corev1.ResourceCPU
	}
	m.selectAliases()
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	_ = bad // malformed files are intentionally ignored; shown after refresh status via count only in future detail view.
}

func validateAliases(rules []aliasRule) error {
	for i, rule := range rules {
		if rule.Alias == "" || rule.Resource == "" {
			return fmt.Errorf("rule %d needs alias and resource", i+1)
		}
		if rule.Unit < 0 {
			return fmt.Errorf("rule %d unit must be positive", i+1)
		}
	}
	return nil
}
func defaultConfigPath() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "kube-resource-top", "config.json")
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".config", "kube-resource-top", "config.json")
}
func loadConfig(path string) (c config) {
	b, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(b, &c)
	}
	return
}
func selectTargets(cfg config, targets []target) (map[string]bool, bool) {
	selected := map[string]bool{}
	known := map[string]bool{}
	for _, id := range cfg.KnownTargets {
		known[id] = true
	}
	first := len(cfg.Targets) == 0 && len(known) == 0
	for _, t := range targets {
		if first || !known[t.ID] {
			selected[t.ID] = true
		}
	}
	for _, t := range targets {
		for _, id := range cfg.Targets {
			if id == t.ID {
				selected[id] = true
			}
		}
	}
	return selected, first
}
func (m model) save() {
	var ids []string
	for id, on := range m.selected {
		if on {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	m.cfg.Targets = ids
	m.cfg.KnownTargets = nil
	for _, target := range m.targets {
		m.cfg.KnownTargets = append(m.cfg.KnownTargets, target.ID)
	}
	sort.Strings(m.cfg.KnownTargets)
	m.cfg.Resources = nil
	for r, on := range m.resourceSelected {
		if on {
			m.cfg.Resources = append(m.cfg.Resources, string(r))
		}
	}
	sort.Strings(m.cfg.Resources)
	_ = os.MkdirAll(filepath.Dir(m.cfgPath), 0700)
	if b, e := json.MarshalIndent(m.cfg, "", "  "); e == nil {
		_ = os.WriteFile(m.cfgPath, b, 0600)
	}
}

type checkResult struct{ path, context, err string }

// testKubeconfigs checks parsing, credentials, and an authenticated API discovery call for every context.
func testKubeconfigs(dir string, timeout time.Duration, concurrency int) int {
	if timeout <= 0 || concurrency < 1 {
		fmt.Fprintln(os.Stderr, "test timeout must be positive and concurrency must be at least 1")
		return 2
	}
	root, err := filepath.Abs(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	var jobs []target
	var results, available []checkResult
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !isKubeconfigCandidate(path) {
			return nil
		}
		raw, loadErr := clientcmd.LoadFromFile(path)
		if loadErr != nil {
			results = append(results, checkResult{path: path, err: loadErr.Error()})
			return nil
		}
		if len(raw.Contexts) == 0 {
			results = append(results, checkResult{path: path, err: "no contexts"})
			return nil
		}
		for name := range raw.Contexts {
			jobs = append(jobs, target{Path: path, Context: name})
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	work := make(chan target)
	var wg sync.WaitGroup
	var mu sync.Mutex
	for range min(concurrency, len(jobs)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range work {
				checkErr := probeTarget(t, timeout)
				mu.Lock()
				if checkErr != nil {
					results = append(results, checkResult{path: t.Path, context: t.Context, err: checkErr.Error()})
				} else {
					available = append(available, checkResult{path: t.Path, context: t.Context})
				}
				mu.Unlock()
			}
		}()
	}
	for _, job := range jobs {
		work <- job
	}
	close(work)
	wg.Wait()
	sort.Slice(available, func(i, j int) bool {
		return available[i].context+available[i].path < available[j].context+available[j].path
	})
	for _, ok := range available {
		fmt.Printf("OK   context: %s  (%s)\n", display(ok.context), ok.path)
	}
	if len(results) == 0 {
		fmt.Println("All kubeconfig contexts are usable.")
		return 0
	}
	sort.Slice(results, func(i, j int) bool { return results[i].path+results[i].context < results[j].path+results[j].context })
	for _, r := range results {
		if r.context == "" {
			fmt.Printf("FAIL %s: %s\n", r.path, r.err)
		} else {
			fmt.Printf("FAIL %s :: %s: %s\n", r.path, display(r.context), r.err)
		}
	}
	return 1
}

func display(s string) string { return strings.TrimSpace(strings.ReplaceAll(s, `\n`, "")) }

func probeTarget(t target, timeout time.Duration) error {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = t.Path
	rc, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{CurrentContext: t.Context}).ClientConfig()
	if err != nil {
		return err
	}
	rc.Timeout = timeout
	client, err := kubernetes.NewForConfig(rc)
	if err != nil {
		return err
	}
	_, err = client.Discovery().ServerVersion()
	return err
}

// Keep extensionless kubeconfigs, while avoiding source files when the binary lives beside its input directory.
func isKubeconfigCandidate(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	if name == "readme" || name == "kube-resource-top" {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	return ext == "" || ext == ".yaml" || ext == ".yml" || ext == ".json"
}

func discover(dir string) ([]target, int, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, 0, err
	}
	var out []target
	bad := 0
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !isKubeconfigCandidate(path) {
			return nil
		}
		raw, e := clientcmd.LoadFromFile(path)
		if e != nil {
			bad++
			return nil
		}
		for name, cx := range raw.Contexts {
			if cx == nil {
				continue
			}
			cl := raw.Clusters[cx.Cluster]
			server := ""
			if cl != nil {
				server = cl.Server
			}
			out = append(out, target{ID: path + "::" + name, Path: path, Context: name, Server: server})
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, bad, err
}

func (m *model) selectAliases() {
	for _, rule := range m.aliases {
		alias := corev1.ResourceName(rule.Alias)
		m.resourceSelected[alias] = true
		if m.resource == corev1.ResourceName(rule.Resource) {
			m.resource = alias
		}
	}
}
func (m *model) restartWatchers() tea.Cmd {
	if m.cancelWatch != nil {
		m.cancelWatch()
	}
	m.watchCtx, m.cancelWatch = context.WithCancel(context.Background())
	m.updates = make(chan resultMsg, 1)
	m.snaps = map[string]snapshot{}
	return m.startWatchers()
}
func (m model) Init() tea.Cmd {
	if m.choosing {
		return m.probeTargets()
	}
	return m.startWatchers()
}

func (m model) probeTargets() tea.Cmd {
	var cmds []tea.Cmd
	sem := make(chan struct{}, 8)
	for _, t := range m.targets {
		t := t
		cmds = append(cmds, func() tea.Msg {
			sem <- struct{}{}
			defer func() { <-sem }()
			return probeMsg{id: t.ID, err: probeTarget(t, 3*time.Second)}
		})
	}
	return tea.Batch(cmds...)
}
func (m model) startWatchers() tea.Cmd {
	for _, t := range m.targets {
		if m.selected[t.ID] {
			go watchTarget(m.watchCtx, t, m.aliases, m.updates)
		}
	}
	return m.waitUpdate()
}
func (m model) waitUpdate() tea.Cmd { return func() tea.Msg { return <-m.updates } }
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch x := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = x.Width, x.Height
	case tea.KeyMsg:
		k := x.String()
		if m.choosing {
			switch k {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.targets)-1 {
					m.cursor++
				}
			case " ":
				t := m.targets[m.cursor]
				m.selected[t.ID] = !m.selected[t.ID]
			case "enter":
				if m.probePending > 0 {
					return m, nil
				}
				m.choosing = false
				m.save()
				return m, m.restartWatchers()
			}
			return m, nil
		}
		if k == "ctrl+c" {
			m.cancelWatch()
			m.save()
			return m, tea.Quit
		}
		if m.aliasCreating {
			return m.updateAlias(k)
		}
		if m.resourcePicking {
			resources := m.availableResources()
			switch k {
			case "esc":
				m.resourcePicking = false
			case "up", "k":
				if m.resourceCursor > 0 {
					m.resourceCursor--
				}
			case "down", "j":
				if m.resourceCursor < len(resources)-1 {
					m.resourceCursor++
				}
			case " ":
				r := resources[m.resourceCursor]
				m.resourceSelected[r] = !m.resourceSelected[r]
			case "enter":
				if len(m.selectedResources()) > 0 {
					m.resourcePicking = false
					m.save()
				}
			}
			return m, nil
		}
		if m.searching {
			switch k {
			case "enter", "esc":
				m.searching = false
			case "backspace":
				if len(m.search) > 0 {
					m.search = m.search[:len(m.search)-1]
				}
			default:
				if len(k) == 1 {
					m.search += k
				}
			}
			m.clampCursor()
			return m, nil
		}
		switch k {
		case "q", "ctrl+c":
			m.cancelWatch()
			m.save()
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < m.rowCount()-1 {
				m.cursor++
			}
		case "pgdown", "ctrl+d":
			m.cursor += m.pageSize()
			m.clampCursor()
		case "pgup", "ctrl+u":
			m.cursor -= m.pageSize()
			if m.cursor < 0 {
				m.cursor = 0
			}
		case "/":
			m.searching = true
		case "enter":
			rs := m.filteredDisplayRows()
			if !m.nodeBalance && m.viewMode != 2 && len(rs) > 0 {
				m.resource = rs[m.cursor].resource
				m.scope = append(m.scope, rs[m.cursor].key)
				m.viewMode = 0
				m.cursor = 0
			}
		case "backspace", "esc":
			if m.nodeBalance {
				m.nodeBalance = false
				m.cursor = 0
				m.search, m.searching = "", false
			} else if len(m.scope) > 0 {
				m.scope = m.scope[:len(m.scope)-1]
				m.cursor = 0
			}
		case "r":
			// Watch streams reconnect through client-go; there is no polling refresh.
		case "1":
			m.resource = corev1.ResourceCPU
			m.resourceSelected[m.resource] = true
		case "2":
			m.resource = corev1.ResourceMemory
			m.resourceSelected[m.resource] = true
		case "3", "tab":
			if m.nodeBalance {
				m.resource = m.nextSelectedResource()
			} else {
				m.resource = m.nextResource()
			}
			m.resourceSelected[m.resource] = true
		case "c":
			m.choosing, m.probePending = true, len(m.targets)
			for i := range m.targets {
				m.targets[i].Probe = ""
			}
			return m, m.probeTargets()
		case "a":
			m.aliasCreating, m.aliasEditing, m.aliasCursor, m.aliasError = true, false, 0, ""
		case "m":
			m.resourcePicking = true
			for i, r := range m.availableResources() {
				if r == m.resource {
					m.resourceCursor = i
				}
			}
		case "v":
			if m.nodeBalance {
				m.nodeScope = (m.nodeScope + 1) % 4
			} else {
				m.viewMode = (m.viewMode + 1) % 3
			}
			m.cursor = 0
		case "b":
			m.nodeBalance = !m.nodeBalance
			if m.nodeBalance {
				m.nodeScope = nodeScopeDrill
			}
			m.cursor = 0
			m.search, m.searching = "", false // ponytail: stale drill-down search would hide every node in the balance view
			if m.nodeBalance {
				// Enter the nodes view on a resource the user actually selected;
				// otherwise it shows whichever key happened to be active.
				if sel := m.selectedResources(); len(sel) > 0 {
					selected := false
					for _, r := range sel {
						if r == m.resource {
							selected = true
							break
						}
					}
					if !selected {
						m.resource = sel[0]
					}
				}
			}
		case "s":
			m.sortBy = (m.sortBy + 1) % 5
		case "S":
			m.reverse = !m.reverse
		}
	case probeMsg:
		for i := range m.targets {
			if m.targets[i].ID != x.id {
				continue
			}
			if x.err == nil {
				m.targets[i].Probe = "available"
			} else {
				m.targets[i].Probe = "unavailable"
				m.selected[x.id] = false
			}
			break
		}
		m.probePending--
		if m.probePending == 0 {
			sort.SliceStable(m.targets, func(i, j int) bool { return m.targets[i].Probe == "available" && m.targets[j].Probe != "available" })
		}
	case resultMsg:
		m.snaps[x.s.Target.ID] = x.s
		return m, m.waitUpdate()
	}
	return m, nil
}

type aliasOption struct{ key, value string }

func (m model) aliasOptions(annotations bool) []aliasOption {
	seen := map[string]bool{}
	var out []aliasOption
	for _, snapshot := range m.snaps {
		for _, node := range snapshot.Nodes {
			values := node.Labels
			if annotations {
				values = node.Annotations
			}
			for key, value := range values {
				id := key + "\x00" + value
				if !seen[id] {
					seen[id] = true
					out = append(out, aliasOption{key, value})
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key+"\x00"+out[i].value < out[j].key+"\x00"+out[j].value })
	return out
}
func (m *model) pickAliasOptions(picking, returnField int) {
	m.aliasPicking, m.aliasPickerReturnField, m.aliasCursor = picking, returnField, 0
	m.aliasConditions = map[string]bool{}
	field := 2
	if picking == 2 {
		field = 3
	}
	values, _ := parseConditions(m.aliasFields[field])
	for _, option := range m.aliasOptions(picking == 2) {
		if values[option.key] == option.value {
			m.aliasConditions[option.key+"\x00"+option.value] = true
		}
	}
}
func (m *model) storeAliasOptions(annotations bool) {
	values := map[string]string{}
	for _, option := range m.aliasOptions(annotations) {
		if m.aliasConditions[option.key+"\x00"+option.value] {
			values[option.key] = option.value
		}
	}
	if annotations {
		m.aliasFields[3] = conditionText(values)
	} else {
		m.aliasFields[2] = conditionText(values)
	}
}
func conditionText(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values[key])
	}
	return strings.Join(parts, ",")
}
func parseConditions(text string) (map[string]string, error) {
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	out := map[string]string{}
	for _, part := range strings.Split(text, ",") {
		key, value, ok := strings.Cut(part, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ok || key == "" {
			return nil, fmt.Errorf("conditions must be comma-separated key=value pairs")
		}
		if _, exists := out[key]; exists {
			return nil, fmt.Errorf("condition %q is repeated", key)
		}
		out[key] = value
	}
	return out, nil
}
func (m model) editorAliasRule() (aliasRule, error) {
	labels, err := parseConditions(m.aliasFields[2])
	if err != nil {
		return aliasRule{}, fmt.Errorf("labels: %w", err)
	}
	annotations, err := parseConditions(m.aliasFields[3])
	if err != nil {
		return aliasRule{}, fmt.Errorf("annotations: %w", err)
	}
	unit := int64(1)
	if text := strings.TrimSpace(m.aliasFields[4]); text != "" {
		unit, err = strconv.ParseInt(text, 10, 64)
		if err != nil || unit < 1 {
			return aliasRule{}, fmt.Errorf("unit must be a positive integer")
		}
	}
	return aliasRule{Alias: strings.TrimSpace(m.aliasFields[0]), Resource: strings.TrimSpace(m.aliasFields[1]), Labels: labels, Annotations: annotations, Unit: unit}, nil
}
func (m *model) beginAliasEdit(index int) {
	m.aliasEditing, m.aliasEditIndex, m.aliasField, m.aliasError = true, index, 0, ""
	if index < 0 {
		m.aliasFields = [5]string{"", string(m.resource), "", "", "1"}
		return
	}
	rule := m.aliases[index]
	unit := rule.Unit
	if unit == 0 {
		unit = 1
	}
	m.aliasFields = [5]string{rule.Alias, rule.Resource, conditionText(rule.Labels), conditionText(rule.Annotations), strconv.FormatInt(unit, 10)}
}
func (m *model) updateAlias(k string) (tea.Model, tea.Cmd) {
	if m.aliasSelectingResource {
		resources := m.availableResources()
		switch k {
		case "esc":
			m.aliasSelectingResource = false
		case "up", "k":
			if m.aliasCursor > 0 {
				m.aliasCursor--
			}
		case "down", "j":
			if m.aliasCursor < len(resources)-1 {
				m.aliasCursor++
			}
		case "enter":
			selected := resources[m.aliasCursor]
			m.aliasSelectingResource, m.aliasEditIndex = false, -1
			m.aliasFields = [5]string{"", string(selected), "", "", "1"}
			m.pickAliasOptions(1, -1)
		}
		return *m, nil
	}
	if m.aliasPicking > 0 {
		annotations := m.aliasPicking == 2
		options := m.aliasOptions(annotations)
		switch k {
		case "esc":
			m.aliasPicking = 0
			if m.aliasPickerReturnField >= 0 {
				m.aliasEditing, m.aliasField = true, m.aliasPickerReturnField
			} else {
				m.aliasSelectingResource = true
			}
		case "up", "k":
			if m.aliasCursor > 0 {
				m.aliasCursor--
			}
		case "down", "j":
			if m.aliasCursor < len(options)-1 {
				m.aliasCursor++
			}
		case "pgdown", "ctrl+d":
			m.aliasCursor = min(m.aliasCursor+m.pageSize(), max(0, len(options)-1))
		case "pgup", "ctrl+u":
			m.aliasCursor = max(0, m.aliasCursor-m.pageSize())
		case " ":
			if len(options) > 0 {
				key := options[m.aliasCursor].key + "\x00" + options[m.aliasCursor].value
				m.aliasConditions[key] = !m.aliasConditions[key]
			}
		case "enter":
			m.storeAliasOptions(annotations)
			returnField := m.aliasPickerReturnField
			m.aliasConditions = map[string]bool{}
			if returnField >= 0 {
				m.aliasPicking, m.aliasEditing, m.aliasField = 0, true, returnField
			} else if annotations {
				m.aliasPicking, m.aliasEditing, m.aliasEditIndex, m.aliasField = 0, true, -1, 0
			} else {
				m.pickAliasOptions(2, -1)
			}
		}
		return *m, nil
	}
	if !m.aliasEditing {
		switch k {
		case "esc":
			m.aliasCreating = false
		case "up", "k":
			if m.aliasCursor > 0 {
				m.aliasCursor--
			}
		case "down", "j":
			if m.aliasCursor < len(m.aliases)-1 {
				m.aliasCursor++
			}
		case "a":
			m.aliasSelectingResource, m.aliasCursor, m.aliasError = true, 0, ""
		case "enter":
			if len(m.aliases) > 0 {
				m.beginAliasEdit(m.aliasCursor)
			}
		}
		return *m, nil
	}
	if k == "esc" {
		m.aliasEditing = false
		m.aliasError = ""
		return *m, nil
	}
	switch k {
	case "up", "k":
		if m.aliasField > 0 {
			m.aliasField--
		}
	case "down", "j", "tab":
		if m.aliasField < len(m.aliasFields)-1 {
			m.aliasField++
		}
	case "backspace":
		r := []rune(m.aliasFields[m.aliasField])
		if len(r) > 0 {
			m.aliasFields[m.aliasField] = string(r[:len(r)-1])
		}
	case "enter":
		if m.aliasField == 2 {
			m.pickAliasOptions(1, 2)
		} else if m.aliasField == 3 {
			m.pickAliasOptions(2, 3)
		} else {
			m.aliasField = (m.aliasField + 1) % len(m.aliasFields)
		}
	case "ctrl+s":
		rule, err := m.editorAliasRule()
		if err == nil {
			rules := append([]aliasRule{}, m.aliases...)
			if m.aliasEditIndex < 0 {
				rules = append(rules, rule)
			} else {
				rules[m.aliasEditIndex] = rule
			}
			err = validateAliases(rules)
			if err == nil {
				if m.aliasEditIndex >= 0 && m.resource == corev1.ResourceName(m.aliases[m.aliasEditIndex].Alias) {
					m.resource = corev1.ResourceName(rule.Alias)
				}
				m.aliases, m.cfg.Aliases = rules, rules
				m.selectAliases()
				m.save()
				m.aliasCreating, m.aliasEditing = false, false
				return *m, m.restartWatchers()
			}
		}
		m.aliasError = err.Error()
	default:
		if len([]rune(k)) == 1 {
			m.aliasFields[m.aliasField] += k
		}
	}
	return *m, nil
}

type nodeRow struct {
	name, targetID, context, nodeName string
	resource                          corev1.ResourceName
	capacity, requested, pending      resource.Quantity
}

func (m model) nodeResources() []corev1.ResourceName {
	switch m.nodeScope {
	case nodeScopeAll:
		return m.availableResources()
	case nodeScopeResources:
		return m.selectedResources()
	default:
		return []corev1.ResourceName{m.resource}
	}
}
func (m model) nodeIncludesTarget(id string) bool {
	if m.nodeScope == nodeScopeDrill && len(m.scope) > 0 {
		return id == m.scope[0]
	}
	if len(m.selected) == 0 { // snapshots without the interactive selector
		return true
	}
	return m.selected[id]
}
func (m model) nodeRows() []nodeRow {
	var out []nodeRow
	for _, snapshot := range m.snaps {
		if snapshot.Err != "" || !m.nodeIncludesTarget(snapshot.Target.ID) {
			continue
		}
		for _, resourceName := range m.nodeResources() {
			var pending resource.Quantity
			for _, pod := range snapshot.Pods {
				if pod.Phase == corev1.PodPending && !pod.Offloaded {
					pending.Add(pod.Requests[resourceName])
				}
			}
			rows := map[string]*nodeRow{}
			for _, node := range snapshot.Nodes {
				capacity := node.Capacity[resourceName]
				if capacity.IsZero() {
					continue
				}
				name := node.IP
				if name == "" {
					name = node.Name
				}
				rows[node.Name] = &nodeRow{name: name, targetID: snapshot.Target.ID, context: display(snapshot.Target.Context), nodeName: node.Name, resource: resourceName, capacity: capacity, pending: pending}
			}
			for _, pod := range snapshot.Pods {
				if pod.Phase == corev1.PodRunning && pod.NodeName != "" && rows[pod.NodeName] != nil {
					rows[pod.NodeName].requested.Add(pod.Requests[resourceName])
				}
			}
			for _, row := range rows {
				out = append(out, *row)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].context+"\x00"+string(out[i].resource)+"\x00"+out[i].name < out[j].context+"\x00"+string(out[j].resource)+"\x00"+out[j].name
	})
	if m.search == "" {
		return out
	}
	filtered := out[:0]
	for _, row := range out {
		if strings.Contains(strings.ToLower(row.name+" "+row.context+" "+string(row.resource)), strings.ToLower(m.search)) {
			filtered = append(filtered, row)
		}
	}
	return filtered
}
func (m model) nodeVisibleRows() ([]nodeRow, int, int) {
	rows := m.nodeRows()
	start := min(m.cursor/m.nodePageSize()*m.nodePageSize(), len(rows))
	end := min(start+m.nodePageSize(), len(rows))
	return rows[start:end], start, end
}

func (m model) nodePageSize() int { return max(3, (m.height-8)/2) }

// nodeFragMax bounds how many per-pod request lines the fragmentation panel shows.
func (m model) nodeFragMax() int { return max(3, m.height-8-m.nodePageSize()) }

// nodePendingPods returns pending pods in the given cluster that request the
// active resource (they have no assigned Node, so they are cluster-level demand).
func (m model) nodePendingPods(targetID string, resourceName corev1.ResourceName) []pod {
	for _, snapshot := range m.snaps {
		if snapshot.Target.ID != targetID {
			continue
		}
		var out []pod
		for _, pod := range snapshot.Pods {
			if pod.Phase != corev1.PodPending || pod.Offloaded {
				continue
			}
			req := pod.Requests[resourceName]
			if req.IsZero() {
				continue
			}
			out = append(out, pod)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Namespace+"/"+out[i].Name < out[j].Namespace+"/"+out[j].Name })
		return out
	}
	return nil
}

// nodePods returns running pods on the given node that request the active
// resource, already alias-scaled to match the node table's capacity column.
func (m model) nodePods(targetID, nodeName string, resourceName corev1.ResourceName) []pod {
	for _, snapshot := range m.snaps {
		if snapshot.Target.ID != targetID {
			continue
		}
		var out []pod
		for _, pod := range snapshot.Pods {
			if pod.Phase != corev1.PodRunning || pod.NodeName != nodeName {
				continue
			}
			req := pod.Requests[resourceName]
			if req.IsZero() {
				continue
			}
			out = append(out, pod)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Namespace+"/"+out[i].Name < out[j].Namespace+"/"+out[j].Name })
		return out
	}
	return nil
}

type row struct {
	key, name        string
	q, u, l, c       resource.Quantity
	running, pending resource.Quantity
	usageOK, capOK   bool
	status           string
}
type displayRow struct {
	row
	resource corev1.ResourceName
}

func (m model) displayRows() []displayRow {
	var out []displayRow
	for _, resource := range m.selectedResources() {
		copy := m
		copy.resource = resource
		for _, row := range copy.rows() {
			out = append(out, displayRow{row, resource})
		}
	}
	if m.sortBy == 4 {
		sort.Slice(out, func(i, j int) bool {
			if out[i].name == out[j].name {
				return out[i].resource < out[j].resource
			}
			return out[i].name < out[j].name
		})
	}
	return out
}
func (m model) filteredDisplayRows() []displayRow {
	rows := m.viewRows()
	if m.search == "" {
		return rows
	}
	query := strings.ToLower(m.search)
	out := rows[:0]
	for _, r := range rows {
		if strings.Contains(strings.ToLower(r.name+" "+string(r.resource)), query) {
			out = append(out, r)
		}
	}
	return out
}
func (m model) viewRows() []displayRow {
	if m.viewMode == 0 {
		return m.displayRows()
	}
	all := m
	all.scope = nil
	rows := all.displayRows()
	if m.viewMode == 1 {
		sort.Slice(rows, func(i, j int) bool {
			a, b := sortValue(rows[i].row, 0), sortValue(rows[j].row, 0)
			if a == b {
				return rows[i].name < rows[j].name
			}
			return a > b
		})
		return rows
	}
	grouped := map[corev1.ResourceName]*row{}
	for _, item := range rows {
		r := grouped[item.resource]
		if r == nil {
			r = &row{name: "All contexts", key: string(item.resource), usageOK: true, capOK: true}
			grouped[item.resource] = r
		}
		r.q.Add(item.q)
		r.u.Add(item.u)
		r.running.Add(item.running)
		r.pending.Add(item.pending)
		r.c.Add(item.c)
		r.usageOK = r.usageOK && item.usageOK
		r.capOK = r.capOK && item.capOK
	}
	out := make([]displayRow, 0, len(grouped))
	for resource, r := range grouped {
		out = append(out, displayRow{*r, resource})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := sortValue(out[i].row, 0), sortValue(out[j].row, 0)
		if a == b {
			return out[i].resource < out[j].resource
		}
		return a > b
	})
	return out
}
func (m model) viewName() string { return []string{"detail", "clusters", "resources"}[m.viewMode] }
func (m model) pageSize() int    { return max(3, m.height-13) }
func (m model) rowCount() int {
	if m.nodeBalance {
		return len(m.nodeRows())
	}
	return len(m.filteredDisplayRows())
}
func (m *model) clampCursor() {
	if m.cursor >= m.rowCount() {
		m.cursor = max(0, m.rowCount()-1)
	}
}
func (m model) visibleRows() ([]displayRow, int, int) {
	rows := m.filteredDisplayRows()
	start := m.cursor / m.pageSize() * m.pageSize()
	if start > len(rows) {
		start = 0
	}
	end := min(start+m.pageSize(), len(rows))
	return rows[start:end], start, end
}

func hasAllocatable(s snapshot, resourceName corev1.ResourceName) bool {
	if s.Capacity == nil {
		return true
	}
	quantity := s.Capacity[resourceName]
	return !quantity.IsZero()
}
func (m model) rows() []row {
	level := len(m.scope)
	grouped := map[string]*row{}
	capacityAdded := map[string]bool{}
	for _, s := range m.snaps {
		if s.Err != "" || !hasAllocatable(s, m.resource) {
			continue
		}
		for _, p := range s.Pods {
			// Offloaded pods (Liqo virtual nodes) run on the remote provider cluster;
			// they are remote work, not local consumer demand, so they are excluded
			// from the context and pod views to avoid inflating request/pending.
			if p.Offloaded {
				continue
			}
			parts := []string{s.Target.ID, p.Namespace, p.Workload, p.Name}
			if level >= len(parts) {
				continue
			}
			match := true
			for i, v := range m.scope {
				if parts[i] != v {
					match = false
				}
			}
			if !match {
				continue
			}
			key := parts[level]
			r := grouped[key]
			if r == nil {
				name := key
				if level == 0 {
					name = filepath.Base(s.Target.Path) + " :: " + display(s.Target.Context)
				}
				r = &row{key: key, name: name, usageOK: true, capOK: level == 0}
				grouped[key] = r
			}
			r.q.Add(p.Requests[m.resource])
			if p.Phase == corev1.PodPending {
				// Offloaded (Liqo virtual-node) pending pods are remote/backoff work,
				// not local consumer demand — don't inflate the pending count.
				if !p.Offloaded {
					r.pending.Add(p.Requests[m.resource])
				}
			} else {
				r.running.Add(p.Requests[m.resource])
			}
			r.l.Add(p.Limits[m.resource])
			if v, ok := p.Usage[m.resource]; ok {
				r.u.Add(v)
			} else {
				r.usageOK = false
			}
			if level == 0 && !capacityAdded[s.Target.ID] {
				capacityAdded[s.Target.ID] = true
				r.c.Add(s.Capacity[m.resource])
				if _, ok := s.Capacity[m.resource]; !ok {
					r.capOK = false
				}
			}
		}
	}
	out := make([]row, 0, len(grouped))
	for _, r := range grouped {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := sortValue(out[i], m.sortBy), sortValue(out[j], m.sortBy)
		less := a > b
		if m.reverse {
			less = !less
		}
		if a == b {
			return out[i].name < out[j].name
		}
		return less
	})
	if m.cursor >= len(out) {
		m.cursor = max(0, len(out)-1)
	}
	return out
}
func sortValue(r row, by int) float64 {
	q := r.q.AsApproximateFloat64()
	if by == 0 {
		if !r.usageOK || q == 0 {
			return -1
		}
		return r.u.AsApproximateFloat64() / q
	}
	if by == 1 {
		if !r.capOK || r.c.IsZero() {
			return -1
		}
		return q / r.c.AsApproximateFloat64()
	}
	if by == 2 {
		return r.u.AsApproximateFloat64()
	}
	if by == 3 {
		return q
	}
	return 0
}
func (m model) availableResources() []corev1.ResourceName {
	seen := map[corev1.ResourceName]bool{corev1.ResourceCPU: true, corev1.ResourceMemory: true}
	for _, s := range m.snaps {
		for _, p := range s.Pods {
			for k := range p.Requests {
				seen[k] = true
			}
		}
		for k := range s.Capacity {
			seen[k] = true
		}
		for _, node := range s.Nodes {
			for k := range node.Capacity {
				seen[k] = true
			}
		}
	}
	resources := make([]corev1.ResourceName, 0, len(seen))
	for k := range seen {
		resources = append(resources, k)
	}
	sort.Slice(resources, func(i, j int) bool { return resources[i] < resources[j] })
	return resources
}
func (m model) selectedResources() []corev1.ResourceName {
	var out []corev1.ResourceName
	for _, r := range m.availableResources() {
		if m.resourceSelected[r] {
			out = append(out, r)
		}
	}
	return out
}
func (m model) nextResource() corev1.ResourceName {
	resources := m.availableResources()
	for i, r := range resources {
		if r == m.resource {
			return resources[(i+1)%len(resources)]
		}
	}
	return resources[0]
}

// nextSelectedResource cycles only through resources the user selected (the
// alias resources they care about), so the nodes view can be scoped to one
// specific resource instead of every discovered key including CPU/memory.
func (m model) nextSelectedResource() corev1.ResourceName {
	resources := m.selectedResources()
	if len(resources) == 0 {
		return m.resource
	}
	for i, r := range resources {
		if r == m.resource {
			return resources[(i+1)%len(resources)]
		}
	}
	return resources[0]
}
func (m model) View() string {
	if m.choosing {
		return m.chooseView()
	}
	if m.aliasCreating {
		return m.aliasView()
	}
	if m.resourcePicking {
		return m.resourceView()
	}
	if m.nodeBalance {
		return m.nodeBalanceView()
	}
	ok, failed := 0, 0
	var errors []string
	for _, t := range m.targets {
		if !m.selected[t.ID] {
			continue
		}
		s, yes := m.snaps[t.ID]
		if yes && s.Err == "" {
			ok++
			if s.Note != "" {
				errors = append(errors, t.Context+": "+s.Note)
			}
		} else if yes {
			failed++
			errors = append(errors, t.Context+": "+s.Err)
		}
	}
	path := "All clusters"
	for i, s := range m.scope {
		path += " / " + m.scopeLabel(i, s)
	}
	selectedNames := make([]string, 0)
	for _, r := range m.selectedResources() {
		selectedNames = append(selectedNames, string(r))
	}
	header := fmt.Sprintf("kube-resource-top  %s\nview: %s  resource: %s  selected: %s  source: Kubernetes Watch  sort: %s%s  clusters: %d/%d\n", path, m.viewName(), m.resource, strings.Join(selectedNames, ","), []string{"usage/request", "request/allocatable", "usage", "request", "name"}[m.sortBy], map[bool]string{true: " desc", false: ""}[m.reverse], ok, len(m.selected))
	var b strings.Builder
	b.WriteString(header)
	b.WriteString(m.aggregateView())
	visible, start, end := m.visibleRows()
	nameW := m.nameWidth(scopeNames(visible), 98, 12)
	if m.width >= 120 {
		b.WriteString("\n" + tableLine("  ", []int{nameW, 22, 21, 10, 12, 12}, "SCOPE", "RESOURCE", "USAGE / REQUEST", "USE%", "RUNNING REQ", "PENDING REQ", "REQUEST / ALLOCATABLE") + "\n")
	} else if m.width >= 85 {
		b.WriteString("\n" + tableLine("  ", []int{nameW, 22, 21, 10}, "SCOPE", "RESOURCE", "USAGE / REQUEST", "USE%", "REQUEST / ALLOCATABLE") + "\n")
	} else {
		b.WriteString("\n" + tableLine("  ", []int{nameW, 16, 21}, "SCOPE", "RESOURCE", "USAGE / REQUEST", "USE%") + "\n")
	}
	for i, r := range visible {
		var line string
		if m.width >= 120 {
			line = tableLine("> ", []int{nameW, 22, 21, 10, 12, 12}, fit(r.name, nameW), trim(string(r.resource), 22), pair(r.u, r.q, r.usageOK), ratio(r.u, r.q, r.usageOK), render(r.running), render(r.pending), capacityPair(r.q, r.c, r.capOK))
		} else if m.width >= 85 {
			line = tableLine("> ", []int{nameW, 22, 21, 10}, fit(r.name, nameW), trim(string(r.resource), 22), pair(r.u, r.q, r.usageOK), ratio(r.u, r.q, r.usageOK), capacityPair(r.q, r.c, r.capOK))
		} else {
			line = tableLine("> ", []int{nameW, 16, 21}, fit(r.name, nameW), trim(string(r.resource), 16), pair(r.u, r.q, r.usageOK), ratio(r.u, r.q, r.usageOK))
		}
		if start+i == m.cursor {
			line = selectedStyle.Render(line)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString(gray.Render(fmt.Sprintf("rows %d-%d/%d  PgUp/PgDn page  / search%s\n", start+1, end, len(m.filteredDisplayRows()), map[bool]string{true: " (" + m.search + ")", false: ""}[m.search != ""])))
	if len(errors) > 0 {
		label := "exceptions"
		if failed > 0 {
			label = fmt.Sprintf("partial: %d/%d clusters", ok, len(m.selected))
		}
		b.WriteString(red.Render("\n"+label+" — "+strings.Join(errors, "; ")) + "\n")
	}
	b.WriteString(gray.Render("↑↓ select  Enter drill  Backspace up  v view  1 CPU  2 memory  3/Tab next  m resources  a alias  b nodes  c contexts  s sort  S reverse  q quit"))
	return b.String()
}
func (m model) scopeLabel(level int, key string) string {
	if level != 0 {
		return key
	}
	for _, t := range m.targets {
		if t.ID == key {
			return filepath.Base(t.Path) + " :: " + display(t.Context)
		}
	}
	return key
}
func (m model) aggregateView() string {
	var b strings.Builder
	b.WriteString("\n  ALL CONTEXTS\n")
	for _, resource := range m.selectedResources() {
		all := m
		all.scope = nil
		all.resource = resource
		total := row{usageOK: true, capOK: true}
		for _, r := range all.rows() {
			total.q.Add(r.q)
			total.u.Add(r.u)
			total.running.Add(r.running)
			total.pending.Add(r.pending)
			total.c.Add(r.c)
			total.usageOK = total.usageOK && r.usageOK
			total.capOK = total.capOK && r.capOK
		}
		b.WriteString(fmt.Sprintf("  %-22s %s  running %s  pending %s  allocatable %s\n", trim(string(resource), 22), pair(total.u, total.q, total.usageOK), render(total.running), render(total.pending), render(total.c)))
	}
	return b.String()
}
func (m model) chooseView() string {
	var b strings.Builder
	b.WriteString("kube-resource-top — select contexts (Space toggles, Enter starts)\n")
	if m.probePending > 0 {
		b.WriteString(fmt.Sprintf("checking network: %d remaining; unavailable contexts will be deselected\n", m.probePending))
	}
	b.WriteString("\n")
	for i, t := range m.targets {
		mark := " "
		if m.selected[t.ID] {
			mark = "x"
		}
		status := t.Probe
		if status == "" {
			status = "checking"
		}
		line := fmt.Sprintf("[%s] %-12s %-18s %s", mark, "["+status+"]", display(t.Context), trim(t.Server, 50))
		if i == m.cursor {
			line = selectedStyle.Render(line)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n↑↓ move  Space toggle  Enter save and start  q quit")
	return b.String()
}
func (m model) aliasView() string {
	var b strings.Builder
	if m.aliasSelectingResource {
		b.WriteString("Create node resource alias — select resource\n\n")
		for i, resource := range m.availableResources() {
			line := string(resource)
			if i == m.aliasCursor {
				line = selectedStyle.Render(line)
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\n↑↓ move  Enter select  Esc list")
		return b.String()
	}
	if m.aliasPicking > 0 {
		annotations := m.aliasPicking == 2
		title := "Select labels"
		if annotations {
			title = "Select annotations"
		}
		b.WriteString("Create node resource alias — " + title + "\n\n")
		options := m.aliasOptions(annotations)
		if len(options) == 0 {
			b.WriteString("Waiting for watched node metadata…\n")
		}
		start := m.aliasCursor / m.pageSize() * m.pageSize()
		end := min(start+m.pageSize(), len(options))
		for i, option := range options[start:end] {
			mark := " "
			if m.aliasConditions[option.key+"\x00"+option.value] {
				mark = "x"
			}
			line := fmt.Sprintf("[%s] %s=%s", mark, option.key, option.value)
			if start+i == m.aliasCursor {
				line = selectedStyle.Render(line)
			}
			b.WriteString(line + "\n")
		}
		b.WriteString(fmt.Sprintf("\nrows %d-%d/%d  ↑↓ move  PgUp/PgDn page  Space toggle  Enter continue  Esc back", start+1, end, len(options)))
		return b.String()
	}
	if !m.aliasEditing {
		b.WriteString("Node resource aliases\n\n")
		if len(m.aliases) == 0 {
			b.WriteString("No aliases configured.\n")
		}
		for i, rule := range m.aliases {
			line := fmt.Sprintf("%s → %s", rule.Resource, rule.Alias)
			if i == m.aliasCursor {
				line = selectedStyle.Render(line)
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\n↑↓ move  Enter edit  a add  Esc close")
		return b.String()
	}
	b.WriteString("Edit node resource alias\n\n")
	if m.aliasError != "" {
		b.WriteString(red.Render(m.aliasError) + "\n\n")
	}
	labels := []string{"Alias", "Resource", "Labels", "Annotations", "Unit"}
	for i, label := range labels {
		line := label + ": " + m.aliasFields[i]
		if i == m.aliasField {
			line = selectedStyle.Render(line + "█")
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\nLabels and annotations: key=value,key=value; Unit: underlying units per displayed alias\n↑↓/Tab field  Enter next  Ctrl+S save  Esc list")
	return b.String()
}
func (m model) nodeScopeName() string {
	return []string{"drill", "all", "contexts", "resources"}[m.nodeScope]
}
func (m model) nodeBalanceView() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("kube-resource-top  nodes: %s  resource: %s\n", m.nodeScopeName(), m.resource))
	rows, start, end := m.nodeVisibleRows()
	nameW := m.nameWidth(nodeNames(rows), 112, 12)
	b.WriteString("\n" + tableLine("  ", []int{nameW, 18, 22, 14, 14, 14, 14}, "NODE", "CONTEXT", "RESOURCE", "ALLOCATABLE", "REQUESTED", "PENDING", "AVAILABLE") + "\n")
	for i, row := range rows {
		available := row.capacity.DeepCopy()
		available.Sub(row.requested)
		line := tableLine("> ", []int{nameW, 18, 22, 14, 14, 14, 14}, fit(row.name, nameW), trim(row.context, 18), trim(string(row.resource), 22), render(row.capacity), render(row.requested), render(row.pending), render(available))
		if start+i == m.cursor {
			line = selectedStyle.Render(line)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString(gray.Render(fmt.Sprintf("rows %d-%d/%d  PgUp/PgDn page  / search\n", start+1, end, len(m.nodeRows()))))
	b.WriteString(m.nodeFragPanel(rows, start))
	b.WriteString(gray.Render("↑↓ select  b/Esc back  v scope  3/Tab next resource  m resources  c contexts  q quit"))
	return b.String()
}

// nodeFragPanel renders the fragmentation detail for the cursor's node: pod
// count and per-pod request, split into running (on this Node) and pending
// (cluster-level, unscheduled), plus total allocated and remaining capacity.
func (m model) nodeFragPanel(visible []nodeRow, start int) string {
	all := m.nodeRows()
	var sel nodeRow
	if m.cursor >= start && m.cursor-start < len(visible) {
		sel = visible[m.cursor-start]
	} else if len(all) > 0 && m.cursor >= 0 && m.cursor < len(all) {
		sel = all[m.cursor]
	} else {
		return ""
	}
	running := m.nodePods(sel.targetID, sel.nodeName, sel.resource)
	pending := m.nodePendingPods(sel.targetID, sel.resource)
	available := sel.capacity.DeepCopy()
	available.Sub(sel.requested)
	var b strings.Builder
	b.WriteString("\n" + gray.Render(fmt.Sprintf("▌ %s  running: %d (req %s)  pending: %d (req %s)  remaining: %s / %s  %s", sel.name, len(running), render(sel.requested), len(pending), render(sel.pending), render(available), render(sel.capacity), ratio(sel.requested, sel.capacity, true))) + "\n")
	maxLines := m.nodeFragMax()
	type entry struct {
		pod    pod
		status string
	}
	var entries []entry
	for _, p := range running {
		entries = append(entries, entry{p, "RUNNING"})
	}
	for _, p := range pending {
		entries = append(entries, entry{p, "PENDING"})
	}
	shown := entries
	more := 0
	if len(shown) > maxLines {
		more = len(shown) - maxLines
		shown = shown[:maxLines]
	}
	names := make([]string, len(shown))
	for i, e := range shown {
		names[i] = e.pod.Namespace + "/" + e.pod.Name
	}
	nameW := m.nameWidth(names, 28, 16)
	for _, e := range shown {
		name := e.pod.Namespace + "/" + e.pod.Name
		status := e.status
		if e.status == "PENDING" {
			status = yellow.Render(status)
		}
		b.WriteString(tableLine("  ", []int{nameW, 9, 12}, fit(name, nameW), status, render(e.pod.Requests[sel.resource])) + "\n")
	}
	if more > 0 {
		b.WriteString(gray.Render(fmt.Sprintf("  +%d more\n", more)))
	}
	return b.String()
}
func (m model) resourceView() string {
	var b strings.Builder
	b.WriteString("Select resources (Space toggles, Enter saves)\n\n")
	for i, r := range m.availableResources() {
		mark := " "
		if m.resourceSelected[r] {
			mark = "x"
		}
		line := fmt.Sprintf("[%s] %s", mark, r)
		if i == m.resourceCursor {
			line = selectedStyle.Render(line)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n↑↓ move  Space toggle  Enter save  Esc cancel")
	return b.String()
}

// tableLine uses terminal display width, not byte length, so ANSI-coloured values keep separators aligned.
func tableLine(prefix string, widths []int, values ...string) string {
	cells := make([]string, len(values))
	for i, value := range values {
		width := 0
		if i < len(widths) {
			width = widths[i]
		}
		cells[i] = value + strings.Repeat(" ", max(0, width-lipgloss.Width(value)))
	}
	return prefix + strings.Join(cells, " | ")
}
func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:max(0, n-1)] + "…"
}

// nameWidth sizes a name column to fit the longest of names, bounded by what
// the terminal leaves after `reserved` chars so complete names show when they
// fit and only truncate when the terminal is too narrow.
func (m model) nameWidth(names []string, reserved, floor int) int {
	w := floor
	for _, n := range names {
		if d := lipgloss.Width(n); d > w {
			w = d
		}
	}
	if max := m.width - reserved; max > floor && w > max {
		w = max
	}
	return w
}

// fit renders s for a name column of width w: full when it fits, trimmed only
// when it would overflow the terminal-bounded column.
func fit(s string, w int) string {
	if lipgloss.Width(s) > w {
		return trim(s, w)
	}
	return s
}

func scopeNames(rows []displayRow) []string {
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.name
	}
	return names
}

func nodeNames(rows []nodeRow) []string {
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.name
	}
	return names
}

func pair(a, b resource.Quantity, ok bool) string {
	if !ok {
		return "N/A / " + render(b)
	}
	return render(a) + " / " + render(b)
}
func capacityPair(a, b resource.Quantity, ok bool) string {
	if !ok {
		return gray.Render("N/A")
	}
	return render(a) + " / " + render(b)
}
func render(q resource.Quantity) string {
	if q.Format == resource.DecimalSI && q.AsApproximateFloat64() < 1 && !q.IsZero() {
		return fmt.Sprintf("%dm", q.MilliValue())
	}
	return q.String()
}
func ratio(a, b resource.Quantity, ok bool) string {
	if !ok || b.IsZero() {
		return gray.Render("N/A")
	}
	v := 100 * a.AsApproximateFloat64() / b.AsApproximateFloat64()
	s := fmt.Sprintf("%.0f%%", v)
	if v > 100 {
		return red.Render(s)
	}
	if v >= 80 {
		return yellow.Render(s)
	}
	return s
}

func restConfig(t target) (*rest.Config, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = t.Path
	over := &clientcmd.ConfigOverrides{CurrentContext: t.Context}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, over).ClientConfig()
}
