# Read-only web dashboard over the watch state

`--web ADDR` starts a read-only dashboard instead of the TUI. Cluster state
still comes exclusively from the same `watchTarget` snapshot pipeline
(ADRs 0001, 0026): the browser polls `/api/state.json` on a timer, which is
a view refresh in ADR 0027's VIEW-versus-STATE sense, not a second state
source. Aggregates reuse the `accountedResources()` and level-0 `rows()`
accounting path (ADRs 0003, 0028, 0029, 0032, 0033 apply unchanged), so the
web page and the TUI can never disagree; per-cluster sums are restated from
the same snapshot and pinned by tests.

The dashboard serves the two user types directly as two views. The capacity
view shows each resource type as the aggregate line in ADR 0029's order
(total → req/alloc → running → pending → allocatable) with expandable
per-cluster and per-node tables. The queue view groups each cluster's
unscheduled Pending Pods by workload (group age from the earliest pod, per
ADR 0025's oldest-pod logic), orders them by creation time, and reports the
position, the demand still ahead, the head's wait, and a saturation verdict
built only from observed evidence: full (no remaining allocatable),
fragmented (free cards exist but the largest single-node free amount cannot
fit the head pod), blocked (a fitting node exists yet the head stays
unscheduled), absent, or empty. No ETA is fabricated — the watch state has
no completion history, so wait time is presented as evidence (queue depth,
head age, free capacity), never as a prediction.

The queue view explicitly lists clusters that report zero allocatable
capacity for a resource yet queue demand for it ("cluster has none of this
resource"), narrowing ADR 0013 for the queue surface only: a user asking
"why isn't my task running" must not get silence. The TUI detail rows keep
ADR 0013 behavior. Position is creation-time order, an approximation the
page states openly — priority and affinity can jump the queue. The web page
is read-only: alias authoring (ADRs 0006/0007) and target selection
(ADR 0019) remain TUI-only.
