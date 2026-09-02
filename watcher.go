package main

import (
	"context"
	"strconv"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// watchTarget relies on client-go informers: an initial List establishes state,
// then the Kubernetes Watch stream keeps it current. There is no refresh timer.
func watchTarget(ctx context.Context, t target, aliases []aliasRule, updates chan<- resultMsg) {
	rc, err := restConfig(t)
	if err != nil {
		sendSnapshot(updates, snapshot{Target: t, Err: err.Error()})
		return
	}
	client, err := kubernetes.NewForConfig(rc)
	if err != nil {
		sendSnapshot(updates, snapshot{Target: t, Err: err.Error()})
		return
	}
	state := &watchState{target: t, aliases: aliases, pods: map[string]pod{}, nodes: map[string]corev1.Node{}}
	factory := informers.NewSharedInformerFactory(client, 0)
	pods := factory.Core().V1().Pods().Informer()
	nodes := factory.Core().V1().Nodes().Informer()
	update := func() { sendSnapshot(updates, state.snapshot()) }
	pods.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(v interface{}) {
			if p, ok := v.(*corev1.Pod); ok {
				state.setPod(p)
				update()
			}
		},
		UpdateFunc: func(_, v interface{}) {
			if p, ok := v.(*corev1.Pod); ok {
				state.setPod(p)
				update()
			}
		},
		DeleteFunc: func(v interface{}) { state.deletePod(v); update() },
	})
	nodes.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(v interface{}) {
			if n, ok := v.(*corev1.Node); ok {
				state.setNode(n)
				update()
			}
		},
		UpdateFunc: func(_, v interface{}) {
			if n, ok := v.(*corev1.Node); ok {
				state.setNode(n)
				update()
			}
		},
		DeleteFunc: func(v interface{}) { state.deleteNode(v); update() },
	})
	factory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), pods.HasSynced, nodes.HasSynced) {
		return
	}
	update()
	<-ctx.Done()
}

type watchState struct {
	mu      sync.Mutex
	target  target
	aliases []aliasRule
	pods    map[string]pod
	nodes   map[string]corev1.Node
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
	if p.Status.Phase == corev1.PodPending {
		requests = pendingNPURequests(requests, p.Labels)
	}
	s.pods[key] = pod{Namespace: p.Namespace, Name: p.Name, UID: string(p.UID), NodeName: p.Spec.NodeName, Phase: p.Status.Phase, OwnerRefs: p.OwnerReferences, Workload: ownerForPod(p.Namespace, p.Name, p.OwnerReferences, nil), Requests: requests, Limits: podQuantities(*p, true), Annotations: p.Annotations}
}
func pendingNPURequests(requests qtys, labels map[string]string) qtys {
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
func (s *watchState) deletePod(v interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := v.(*corev1.Pod); ok {
		delete(s.pods, p.Namespace+"/"+p.Name)
	}
}
func (s *watchState) setNode(n *corev1.Node) { s.mu.Lock(); defer s.mu.Unlock(); s.nodes[n.Name] = *n }
func (s *watchState) deleteNode(v interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n, ok := v.(*corev1.Node); ok {
		delete(s.nodes, n.Name)
	}
}
func (s *watchState) snapshot() snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := snapshot{Target: s.target, Capacity: qtys{}}
	pendingAliases := s.singleAliasRules()
	for _, raw := range s.pods {
		p := raw
		// A pod is offloaded to a remote provider when Liqo marked it
		// (a liqo.io/ annotation) or scheduled it onto a virtual node.
		for k := range p.Annotations {
			if strings.HasPrefix(k, "liqo.io/") {
				p.Offloaded = true
				break
			}
		}
		if !p.Offloaded && p.NodeName != "" {
			if node, ok := s.nodes[p.NodeName]; ok {
				p.Offloaded = node.Labels["liqo.io/type"] == "virtual-node"
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
		capacity := aliasQuantities(qtys(n.Status.Allocatable), n, s.aliases)
		out.Nodes = append(out.Nodes, nodeInfo{Name: n.Name, IP: nodeIP(n), Labels: n.Labels, Annotations: n.Annotations, Capacity: capacity})
		for resource, quantity := range capacity {
			value := out.Capacity[resource]
			value.Add(quantity)
			out.Capacity[resource] = value
		}
	}
	return out
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
func sendSnapshot(ch chan<- resultMsg, s snapshot) {
	select {
	case ch <- resultMsg{s: s}:
	default:
	}
}
