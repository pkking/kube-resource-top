# kube-resource-top

Keyboard-only multi-cluster Kubernetes resource TUI.

```sh
go run . --kubeconfig-dir /path/to/kubeconfigs

# Check parsing, credentials, and API connectivity without opening the TUI.
go run . -t --kubeconfig-dir /path/to/kubeconfigs
# Tune parallelism and per-context deadline (defaults: 8 workers, 3s).
go run . -t --test-concurrency 16 --test-timeout 2s --kubeconfig-dir /path/to/kubeconfigs
```

On first launch, select contexts with `Space` and press `Enter`. It persists the selection in `${XDG_CONFIG_HOME:-~/.config}/kube-resource-top/config.json` (override with `--config`); kubeconfig contexts newly added to the directory are selected automatically on the next launch.

- `Enter` / `Backspace`: drill down or up (cluster → namespace → workload → Pod). Workload rows group Pods by their top-level controller — Deployment, CronJob, StatefulSet, LeaderWorkerSet, or any CRD that manages ReplicaSets, Jobs, or StatefulSets — resolved from controller owner references
- `1`, `2`, `3`/`Tab`: CPU, memory, or cycle discovered resources; `m` opens multi-resource selection
- `b`: toggle per-node view. It opens on the current drill-down context and resource; `v` cycles drill → all nodes/resources → selected contexts → selected resource types. `m` and `c` retain their multi-selections for the latter two scopes. The table shows allocatable, requested, pending, and remaining capacity with a per-pod breakdown; `3`/`Tab` cycles selected resources and `Esc` leaves the view; `s` / `S`: cycle/reverse sort; `q`: quit

The ALL CONTEXTS summary at the top of every view lists each selected resource's total node capacity, request/allocatable ratio, running and pending requests, and allocatable.

`-t` reports every unusable file/context and exits with status 1; it checks kubeconfig parsing, authentication, and Kubernetes API discovery.

The TUI uses Kubernetes List+Watch informers (without a refresh timer) for Pods and Nodes. CPU/memory usage is `N/A` until a streaming metrics source is configured: Metrics Server exposes list/get data, not a reliable Watch stream.

Pods that Liqo offloads to a remote provider cluster are remote work, excluded from their cluster's context, pod-drill, and per-node views. Offloading is decided by placement: a pod scheduled onto a Liqo virtual node (labeled `liqo.io/type=virtual-node`) is remote, and an unscheduled pod carrying a `liqo.io/` annotation counts as offloading-in-progress only while the cluster has virtual nodes. Annotations alone prove nothing — Liqo provider clusters run offloaded work as ShadowPod-owned pods that inherit the consumer's annotations while consuming local devices, and clusters may run or queue their own annotated pods for local capacity. ShadowPod reflections and any pod on a real node are always local demand.

## Node resource aliases

Press `a` in the TUI to list aliases. Press `Enter` to edit the selected rule or `a` again to add one; new rules first select a discovered underlying resource, then select observed node label and annotation conditions. Finally enter the alias and unit. Saving persists the rule and immediately restarts the watches. All conditions must match; multiple rules may use the same resource, and the first matching persisted rule wins.

Rules can also be added directly to the saved config:

```json
{
  "aliases": [{
    "alias": "atlas-800i",
    "resource": "huawei.com/ascend-1980",
    "unit": 2,
    "labels": {"model": "910C"},
    "annotations": {"example.com/pool": "atlas"}
  }]
}
```

`unit` is the number of underlying resource units per displayed alias unit (default: `1`): with `unit: 2`, two `huawei.com/ascend-1980` display as one `atlas-800i`. Matching Node capacity and Running Pod requests are scaled this way; Pending Pods retain the original resource because they have no assigned Node. Pending Pods also contribute a raw request from `ascend-ci.com/npu-resource-model` and positive integer `ascend-ci.com/required-npu-count` labels; an unqualified model such as `ascend-1980` maps to `huawei.com/ascend-1980`. When a cluster has exactly one matching alias for that resource, its Pending demand uses that alias and unit; otherwise it remains raw.

Cards on nodes that match no alias rule stay on the raw resource name. The aggregate and table views surface that raw remainder automatically (alongside the alias lines), so no card-holding pod or idle card pool hides behind an unselected resource; raw and aliased lines are disjoint and nothing is counted twice.
