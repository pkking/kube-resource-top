package main

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAliasMapsMatchingNodeResource(t *testing.T) {
	rules := []aliasRule{{Alias: "atlas-800i", Resource: "huawei.com/ascend-1980", Labels: map[string]string{"model": "910C"}, Unit: 2}}
	node := corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"model": "910C"}}}
	out := aliasQuantities(qtys{corev1.ResourceName("huawei.com/ascend-1980"): resource.MustParse("8")}, node, rules)
	q := out[corev1.ResourceName("atlas-800i")]
	if got := q.Value(); got != 4 {
		t.Fatalf("alias quantity=%d", got)
	}
}
func TestAliasEmptyValueRequiresMetadataKey(t *testing.T) {
	rule := aliasRule{Labels: map[string]string{"node-role.kubernetes.io/npu-a3-560t": ""}}
	if matchesAlias(rule, corev1.Node{}) {
		t.Fatal("missing label must not match an empty-value condition")
	}
	node := corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"node-role.kubernetes.io/npu-a3-560t": ""}}}
	if !matchesAlias(rule, node) {
		t.Fatal("present empty-value label must match")
	}
}

func TestAliasRulesAllowMultipleAliasesForResource(t *testing.T) {
	rules := []aliasRule{{Alias: "a", Resource: "gpu", Labels: map[string]string{"model": "910C"}}, {Alias: "b", Resource: "gpu", Annotations: map[string]string{"rack": "a"}}}
	if err := validateAliases(rules); err != nil {
		t.Fatal(err)
	}
	node := corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"model": "910C"}, Annotations: map[string]string{"rack": "a"}}}
	out := aliasQuantities(qtys{corev1.ResourceName("gpu"): resource.MustParse("2")}, node, rules)
	quantity := out[corev1.ResourceName("a")]
	if got := quantity.Value(); got != 2 {
		t.Fatalf("first matching alias quantity=%d", got)
	}
}
