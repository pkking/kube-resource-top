# Virtual nodes are remote capacity

Liqo virtual nodes (`liqo.io/type=virtual-node`) expose a remote cluster's
capacity; the cards physically live in the provider cluster, which counts them
itself. The watcher now skips virtual nodes when building the snapshot's node
list and its Total and allocatable sums, so a consumer cluster no longer
reports remote pools as its own and the cross-cluster aggregate no longer
double counts them (observed on cn12-001: 624 remote cards — 77% of its
apparent pool — sat on four virtual nodes named after the very clusters that
already report those cards as targets).

This mirrors ADR 0031's pod rule (pods on virtual nodes are offloaded and
excluded) and narrows ADR 0032: the raw-resource surfacing now reflects only
real local leftovers. Virtual nodes stay in the live node map so offload
detection and `hasVirtualNodes` keep working, and they no longer appear in the
per-node view or the alias editor's label picker.
