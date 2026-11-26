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

## Web dashboard

`--web ADDR` serves a read-only dashboard instead of the TUI, on the same watch state:

```sh
go run . --web 0.0.0.0:8080 --kubeconfig-dir /path/to/kubeconfigs
```

The page (Chinese UI) serves two audiences as two tabs:

- **资源总览 (capacity)**: one chip strip per resource type showing total, occupancy (req/alloc), running, pending, and allocatable in ADR 0029's order, expandable to per-cluster and per-node tables. Running and queued regions subdivide by project — read from the GitHub ARC runner labels `actions.github.com/organization`/`repository` on the pods (ADR 0037) — and a donut shows each project's running share; the `ascend-ci.com` NPU labels now count as running demand too, because ARC runner pods hold cards while their scheduler requests are empty.
- **排队进度 (queue)**: per cluster, pending workloads ordered by creation time with queue position, wait age, and demand still ahead, plus a saturation verdict — full / fragmented (free cards exist but no single node fits the head pod) / blocked / cluster has none of this resource. No ETA is fabricated; judge from the head's wait and the free capacity. Queue rows carry the project.

Cluster state still comes exclusively from the Kubernetes Watch; the browser polls `/api/state.json` every 5 s as a view refresh only. Target selection and alias editing remain in the TUI.

The TUI uses Kubernetes List+Watch informers (without a refresh timer) for Pods and Nodes. CPU/memory usage is `N/A` until a streaming metrics source is configured: Metrics Server exposes list/get data, not a reliable Watch stream.

Pods that Liqo offloads to a remote provider cluster are remote work, excluded from their cluster's context, pod-drill, and per-node views. Offloading is decided by placement: a pod scheduled onto a Liqo virtual node (labeled `liqo.io/type=virtual-node`) is remote, and an unscheduled pod carrying a `liqo.io/` annotation counts as offloading-in-progress only while the cluster has virtual nodes. Annotations alone prove nothing — Liqo provider clusters run offloaded work as ShadowPod-owned pods that inherit the consumer's annotations while consuming local devices, and clusters may run or queue their own annotated pods for local capacity. ShadowPod reflections and any pod on a real node are always local demand. Virtual nodes themselves are remote capacity: they are excluded from the totals, the per-node view, and the alias metadata pickers, and the provider cluster that physically holds the cards counts them once.

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
