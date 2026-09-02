package main

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEphemeralRunnerIsDisplayedAsPod(t *testing.T) {
	controller := true
	got := ownerForPod("ns", "runner-pod", []metav1.OwnerReference{{Kind: "EphemeralRunner", Name: "runner", Controller: &controller}}, nil)
	if got != "Pod/runner-pod" {
		t.Fatalf("owner=%q", got)
	}
}

func TestPodQuantitiesUsesSchedulingSemantics(t *testing.T) {
	p := corev1.Pod{Spec: corev1.PodSpec{
		Containers:     []corev1.Container{{Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}}}, {Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")}}}},
		InitContainers: []corev1.Container{{Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")}}}},
		Overhead:       corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10m")},
	}}
	q := podQuantities(p, false)[corev1.ResourceCPU]
	if got := q.MilliValue(); got != 510 {
		t.Fatalf("got %dm, want 510m", got)
	}
}
