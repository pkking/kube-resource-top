# Offload detection by virtual-node placement

Offloaded (remote) work is decided by where a pod runs, not by annotations: a
pod scheduled onto a Liqo virtual node (`liqo.io/type=virtual-node`) is remote
demand and stays excluded from its cluster's views; any pod on a real node is
local demand. A `liqo.io/` annotation (e.g. `liqo.io/api-server-support`) only
marks an unscheduled pod as waiting-to-offload when the cluster actually has
virtual nodes.

The annotation-only rule undercounted real clusters: a Liqo provider runs
offloaded work as ShadowPod-owned pods that inherit the consumer's annotations
while consuming local devices, and clusters run or queue their own annotated
pods locally (observed on one cluster: 96 cards running as annotated
LeaderWorkerSet StatefulSet pods plus 976 cards gang-queued locally while the
TUI showed 32). ShadowPod-owned pods are provider-side reflections and count
as local demand at any phase. Trade-off: while a consumer's peering is down
(no virtual nodes), its stuck annotated pending pods show as local pending
demand instead of being hidden.
