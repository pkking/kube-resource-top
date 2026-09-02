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

- `Enter` / `Backspace`: drill down or up (cluster → namespace → workload → Pod)
- `1`, `2`, `3`/`Tab`: CPU, memory, or cycle discovered resources; `m` opens multi-resource selection
- `b`: toggle per-node view: allocatable, requested, pending, and remaining capacity for the active resource, with a per-pod breakdown (running on the Node plus cluster-wide pending demand); entering the view picks the first selected resource and `3`/`Tab` cycles through the selected resources (use `m` to add more); `Esc` leaves the per-node view; `c`: reselect monitored contexts; `s` / `S`: cycle/reverse sort; `q`: quit

`-t` reports every unusable file/context and exits with status 1; it checks kubeconfig parsing, authentication, and Kubernetes API discovery.

The TUI uses Kubernetes List+Watch informers (without a refresh timer) for Pods and Nodes. CPU/memory usage is `N/A` until a streaming metrics source is configured: Metrics Server exposes list/get data, not a reliable Watch stream.

Pods that Liqo offloads to a remote provider cluster are treated as remote work, not local consumer demand, so they are excluded from the consumer cluster's context, pod-drill, and per-node views (otherwise they'd inflate the consumer's request/pending counts). A pod is detected as offloaded when it carries any `liqo.io/` annotation (e.g. `liqo.io/api-server-support`) — which marks pods intended for offloading, including unscheduled ones still waiting to land on a virtual node — or when it is scheduled onto a Liqo virtual node (labeled `liqo.io/type=virtual-node`).

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
