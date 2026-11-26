package main

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// watchTarget relies on client-go informers: an initial List establishes state,
// then the Kubernetes Watch stream keeps it current. No timer refreshes state;
// the TUI's display-only tick (ADR 0027) just re-renders.
func watchTarget(ctx context.Context, t target, aliases []aliasRule, updates chan<- resultMsg) {
	rc, err := restConfig(t)
	if err != nil {
		sendSnapshot(ctx, updates, snapshot{Target: t, Err: err.Error()})
		return
	}
	client, err := kubernetes.NewForConfig(rc)
	if err != nil {
		sendSnapshot(ctx, updates, snapshot{Target: t, Err: err.Error()})
		return
	}
	state := &watchState{target: t, aliases: aliases, pods: map[string]pod{}, nodes: map[string]corev1.Node{}, owners: ownerMap{}}
	factory := informers.NewSharedInformerFactory(client, 0)
	pods := factory.Core().V1().Pods().Informer()
	nodes := factory.Core().V1().Nodes().Informer()
	replicaSets := factory.Apps().V1().ReplicaSets().Informer()
	jobs := factory.Batch().V1().Jobs().Informer()
	statefulSets := factory.Apps().V1().StatefulSets().Informer()
	dirty := make(chan struct{}, 1)
	notify := func() {
		select {
		case dirty <- struct{}{}:
		default:
		}
	}
	pods.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(v interface{}) {
			if p, ok := v.(*corev1.Pod); ok {
				state.setPod(p)
				notify()
			}
		},
		UpdateFunc: func(_, v interface{}) {
			if p, ok := v.(*corev1.Pod); ok {
				state.setPod(p)
				notify()
			}
		},
		DeleteFunc: func(v interface{}) { state.deletePod(v); notify() },
	})
	nodes.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(v interface{}) {
			if n, ok := v.(*corev1.Node); ok {
				state.setNode(n)
				notify()
			}
		},
		UpdateFunc: func(_, v interface{}) {
			if n, ok := v.(*corev1.Node); ok {
				state.setNode(n)
				notify()
			}
		},
		DeleteFunc: func(v interface{}) { state.deleteNode(v); notify() },
	})
	// Owner informers roll pod owners up to the top workload: ReplicaSet →
	// Deployment, Job → CronJob, StatefulSet → LeaderWorkerSet or any CRD
	// managing StatefulSets. The top owner's kind comes from the owner
	// reference, so the CRD itself is never watched.
	trackOwner := func(kind string, informer cache.SharedIndexInformer) {
		informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    func(v interface{}) { state.setOwner(kind, v); notify() },
			UpdateFunc: func(_, v interface{}) { state.setOwner(kind, v); notify() },
			DeleteFunc: func(v interface{}) { state.deleteOwner(kind, v); notify() },
		})
	}
	trackOwner("ReplicaSet", replicaSets)
	trackOwner("Job", jobs)
	trackOwner("StatefulSet", statefulSets)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-dirty:
			}
			time.Sleep(50 * time.Millisecond) // coalesce event bursts into one snapshot
			sendSnapshot(ctx, updates, state.snapshot())
		}
	}()
	factory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), pods.HasSynced, nodes.HasSynced) {
		return
	}
	// Owner objects are best-effort: without RBAC to list them the cluster
	// still renders, with workloads attributed one level deep.
	grace, cancelGrace := context.WithTimeout(ctx, 2*time.Second)
	cache.WaitForCacheSync(grace.Done(), replicaSets.HasSynced, jobs.HasSynced, statefulSets.HasSynced)
	cancelGrace()
	notify()
	<-ctx.Done()
}

type watchState struct {
	mu      sync.Mutex
	target  target
	aliases []aliasRule
	pods    map[string]pod
	nodes   map[string]corev1.Node
	owners  ownerMap
}

func (s *watchState) setPod(p *corev1.Pod) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := p.Namespace + "/" + p.Name
	if p.Status.Phase != corev1.PodPending && p.Status.Phase != corev1.PodRunning {
		delete(s.pods, key)
		return
	}
	requests := podQuantities(*p, false)
	// The ascend-ci labels express device demand that scheduler requests do
	// not carry — observed on ARC runner pods at both phases (ADR 0037).
	requests = npuLabelRequests(requests, p.Labels)
	// Workload attribution resolves at snapshot time: owner objects may
	// arrive after their pods, and late arrivals re-notify.
	s.pods[key] = pod{Namespace: p.Namespace, Name: p.Name, UID: string(p.UID), NodeName: p.Spec.NodeName, Phase: p.Status.Phase, OwnerRefs: p.OwnerReferences, Requests: requests, Limits: podQuantities(*p, true), Annotations: p.Annotations, Project: projectOfLabels(p.Labels), CreatedAt: p.CreationTimestamp.Time}
}

// npuLabelRequests adds the label-derived extended-resource request
// (ADR 0015) at any phase, keeping max(label, scheduler request) so the
// same demand never counts twice. Running ARC runners hold cards while
// their scheduler requests are empty (ADR 0037).
func npuLabelRequests(requests qtys, labels map[string]string) qtys {
	model := labels["ascend-ci.com/npu-resource-model"]
	count, err := strconv.ParseInt(labels["ascend-ci.com/required-npu-count"], 10, 64)
	if model == "" || err != nil || count < 1 {
		return requests
	}
	resourceName := corev1.ResourceName(model)
	if !strings.Contains(model, "/") {
		resourceName = corev1.ResourceName("huawei.com/" + model)
	}
	labelQuantity := *resource.NewQuantity(count, resource.DecimalSI)
	current := requests[resourceName]
	if current.Cmp(labelQuantity) < 0 {
		requests[resourceName] = labelQuantity
	}
	return requests
}

// projectOfLabels reads the GitHub ARC runner project identity from the
// pod's own labels (ADR 0037): organization/repository when both exist,
// the repository alone otherwise. The labels survive Liqo ShadowPod
// reflection, so offloaded runners keep their project at the provider.
func projectOfLabels(labels map[string]string) string {
	repo := labels["actions.github.com/repository"]
	if repo == "" {
		return ""
	}
	if org := labels["actions.github.com/organization"]; org != "" {
		return org + "/" + repo
	}
	return repo
}
func (s *watchState) deletePod(v interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := v.(cache.DeletedFinalStateUnknown); ok {
		v = d.Obj
	}
	if p, ok := v.(*corev1.Pod); ok {
		delete(s.pods, p.Namespace+"/"+p.Name)
	}
}
func (s *watchState) setNode(n *corev1.Node) { s.mu.Lock(); defer s.mu.Unlock(); s.nodes[n.Name] = *n }
func (s *watchState) deleteNode(v interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := v.(cache.DeletedFinalStateUnknown); ok {
		v = d.Obj
	}
	if n, ok := v.(*corev1.Node); ok {
		delete(s.nodes, n.Name)
	}
}

// setOwner records the top workload for an intermediate owner object
// (ReplicaSet, Job, StatefulSet) from its controller owner reference.
func (s *watchState) setOwner(kind string, v interface{}) {
	if d, ok := v.(cache.DeletedFinalStateUnknown); ok {
		v = d.Obj
	}
	if o, ok := v.(metav1.Object); ok {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.owners == nil {
			s.owners = ownerMap{}
		}
		s.owners[kind+"/"+o.GetNamespace()+"/"+o.GetName()] = topOwner(o.GetOwnerReferences(), kind, o.GetName())
	}
}
func (s *watchState) deleteOwner(kind string, v interface{}) {
	if d, ok := v.(cache.DeletedFinalStateUnknown); ok {
		v = d.Obj
	}
	if o, ok := v.(metav1.Object); ok {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.owners, kind+"/"+o.GetNamespace()+"/"+o.GetName())
	}
}
func (s *watchState) snapshot() snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := snapshot{Target: s.target, Capacity: qtys{}, Total: qtys{}}
	pendingAliases := s.singleAliasRules()
	virtualNodes := s.hasVirtualNodes()
	for _, raw := range s.pods {
		p := raw
		p.Workload = ownerForPod(p.Namespace, p.Name, p.OwnerRefs, s.owners)
		// Offloaded work runs on (or waits for) a Liqo virtual node.
		// Annotations alone prove nothing: Liqo providers run ShadowPod
		// reflections that inherit the consumer's liqo.io/ annotations while
		// consuming local devices, and clusters queue their own annotated
		// pods for local capacity. A liqo.io/ annotation only marks an
		// unscheduled pod as waiting-to-offload in a cluster that has
		// virtual nodes (ADR 0031).
		if p.NodeName != "" {
			if node, ok := s.nodes[p.NodeName]; ok {
				p.Offloaded = node.Labels["liqo.io/type"] == "virtual-node"
			}
		} else if !shadowOwned(p.OwnerRefs) && virtualNodes {
			for k := range p.Annotations {
				if strings.HasPrefix(k, "liqo.io/") {
					p.Offloaded = true
					break
				}
			}
		}
		if p.Phase == corev1.PodPending {
			p.Requests = aliasPendingQuantities(p.Requests, pendingAliases)
		}
		if p.Phase == corev1.PodRunning && p.NodeName != "" {
			if node, ok := s.nodes[p.NodeName]; ok {
				p.Requests = aliasQuantities(p.Requests, node, s.aliases)
				p.Limits = aliasQuantities(p.Limits, node, s.aliases)
			}
		}
		out.Pods = append(out.Pods, p)
	}
	for _, n := range s.nodes {
		if n.Labels["liqo.io/type"] == "virtual-node" {
			// Virtual nodes expose a remote cluster's capacity; the cards live
			// in the provider cluster, which counts them itself (ADR 0033).
			continue
		}
		allocatable := aliasQuantities(qtys(n.Status.Allocatable), n, s.aliases)
		total := aliasQuantities(qtys(n.Status.Capacity), n, s.aliases)
		out.Nodes = append(out.Nodes, nodeInfo{Name: n.Name, IP: nodeIP(n), Labels: n.Labels, Annotations: n.Annotations, Capacity: allocatable})
		sumInto(out.Capacity, allocatable)
		sumInto(out.Total, total)
	}
	return out
}

// sumInto accumulates src quantities into dst.
func sumInto(dst, src qtys) {
	for resourceName, quantity := range src {
		value := dst[resourceName]
		value.Add(quantity)
		dst[resourceName] = value
	}
}

// hasVirtualNodes reports whether this cluster currently has Liqo virtual
// nodes, i.e. whether offloading targets exist at all.
func (s *watchState) hasVirtualNodes() bool {
	for _, n := range s.nodes {
		if n.Labels["liqo.io/type"] == "virtual-node" {
			return true
		}
	}
	return false
}

// shadowOwned reports whether the pod is a Liqo ShadowPod reflection: those
// run in the provider cluster and are local demand at any phase.
func shadowOwned(refs []metav1.OwnerReference) bool {
	for _, r := range refs {
		if r.Kind == "ShadowPod" {
			return true
		}
	}
	return false
}
func nodeIP(node corev1.Node) string {
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			return address.Address
		}
	}
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeExternalIP {
			return address.Address
		}
	}
	return ""
}
func (s *watchState) singleAliasRules() map[corev1.ResourceName]aliasRule {
	matches := map[corev1.ResourceName]map[string]aliasRule{}
	for _, node := range s.nodes {
		if node.Labels["liqo.io/type"] == "virtual-node" {
			continue // remote capacity must not shape local pending attribution
		}
		matched := map[corev1.ResourceName]bool{}
		for _, rule := range s.aliases {
			source := corev1.ResourceName(rule.Resource)
			if matched[source] || !matchesAlias(rule, node) {
				continue
			}
			quantity := node.Status.Allocatable[source]
			if quantity.IsZero() {
				continue
			}
			if matches[source] == nil {
				matches[source] = map[string]aliasRule{}
			}
			matches[source][rule.Alias] = rule
			matched[source] = true
		}
	}
	out := map[corev1.ResourceName]aliasRule{}
	for source, rules := range matches {
		if len(rules) == 1 {
			for _, rule := range rules {
				out[source] = rule
			}
		}
	}
	return out
}
func aliasPendingQuantities(input qtys, rules map[corev1.ResourceName]aliasRule) qtys {
	out := qtys{}
	for resourceName, quantity := range input {
		if rule, ok := rules[resourceName]; ok {
			resourceName = corev1.ResourceName(rule.Alias)
			quantity = scaledAliasQuantity(quantity, rule)
		}
		value := out[resourceName]
		value.Add(quantity)
		out[resourceName] = value
	}
	return out
}
func scaledAliasQuantity(quantity resource.Quantity, rule aliasRule) resource.Quantity {
	if rule.Unit > 1 {
		return *resource.NewMilliQuantity(quantity.MilliValue()/rule.Unit, resource.DecimalSI)
	}
	return quantity
}
func aliasQuantities(input qtys, node corev1.Node, rules []aliasRule) qtys {
	out := qtys{}
	for resourceName, quantity := range input {
		name := resourceName
		for _, rule := range rules {
			if rule.Resource == string(resourceName) && matchesAlias(rule, node) {
				name = corev1.ResourceName(rule.Alias)
				quantity = scaledAliasQuantity(quantity, rule)
				break
			}
		}
		value := out[name]
		value.Add(quantity)
		out[name] = value
	}
	return out
}
func matchesAlias(rule aliasRule, node corev1.Node) bool {
	for key, value := range rule.Labels {
		if actual, ok := node.Labels[key]; !ok || actual != value {
			return false
		}
	}
	for key, value := range rule.Annotations {
		if actual, ok := node.Annotations[key]; !ok || actual != value {
			return false
		}
	}
	return true
}
func sendSnapshot(ctx context.Context, ch chan<- resultMsg, s snapshot) {
	select {
	case ch <- resultMsg{s: s}:
	case <-ctx.Done():
	}
}
