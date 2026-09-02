package main

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func listPods(ctx context.Context, k kubernetes.Interface) ([]pod, error) {
	var out []pod
	var token string
	for {
		list, err := k.CoreV1().Pods("").List(ctx, metav1.ListOptions{Limit: 500, Continue: token})
		if err != nil {
			return nil, err
		}
		for _, p := range list.Items {
			if p.Status.Phase != corev1.PodPending && p.Status.Phase != corev1.PodRunning {
				continue
			}
			out = append(out, pod{Namespace: p.Namespace, Name: p.Name, UID: string(p.UID), OwnerRefs: p.OwnerReferences, Requests: podQuantities(p, false), Limits: podQuantities(p, true)})
		}
		token = list.Continue
		if token == "" {
			return out, nil
		}
	}
}

// podQuantities implements the scheduler's app-sum / init-max / overhead contract.
func podQuantities(p corev1.Pod, limits bool) qtys {
	app, init := qtys{}, qtys{}
	for _, c := range p.Spec.Containers {
		if limits {
			addAll(app, c.Resources.Limits)
		} else {
			addAll(app, c.Resources.Requests)
		}
	}
	for _, c := range p.Spec.InitContainers {
		q := c.Resources.Requests
		if limits {
			q = c.Resources.Limits
		}
		for n, v := range q {
			if old, ok := init[n]; !ok || old.Cmp(v) < 0 {
				init[n] = v.DeepCopy()
			}
		}
	}
	for n, v := range init {
		if old, ok := app[n]; !ok || old.Cmp(v) < 0 {
			app[n] = v
		}
	}
	if p.Spec.Overhead != nil {
		addAll(app, p.Spec.Overhead)
	}
	return app
}
func addAll(dst qtys, src corev1.ResourceList) {
	for n, v := range src {
		x := dst[n]
		x.Add(v)
		dst[n] = x
	}
}

type ownerMap map[string]string

func owners(ctx context.Context, k kubernetes.Interface) (ownerMap, error) {
	rs, e := k.AppsV1().ReplicaSets("").List(ctx, metav1.ListOptions{})
	if e != nil {
		return nil, e
	}
	jobs, e := k.BatchV1().Jobs("").List(ctx, metav1.ListOptions{})
	if e != nil {
		return nil, e
	}
	m := ownerMap{}
	for _, x := range rs.Items {
		m["ReplicaSet/"+x.Namespace+"/"+x.Name] = topOwner(x.OwnerReferences, "ReplicaSet", x.Name)
	}
	for _, x := range jobs.Items {
		m["Job/"+x.Namespace+"/"+x.Name] = topOwner(x.OwnerReferences, "Job", x.Name)
	}
	return m, nil
}
func topOwner(refs []metav1.OwnerReference, fallback, name string) string {
	for _, r := range refs {
		if r.Controller != nil && *r.Controller {
			return r.Kind + "/" + r.Name
		}
	}
	return fallback + "/" + name
}
func ownerName(p pod, m ownerMap) string { return ownerForPod(p.Namespace, p.Name, p.OwnerRefs, m) }
func ownerForPod(namespace, name string, refs []metav1.OwnerReference, m ownerMap) string {
	for _, r := range refs {
		if r.Controller != nil && *r.Controller {
			if r.Kind == "EphemeralRunner" {
				return "Pod/" + name
			}
			if v, ok := m[r.Kind+"/"+namespace+"/"+r.Name]; ok {
				return v
			}
			return r.Kind + "/" + r.Name
		}
	}
	return "Pod/" + name
}
