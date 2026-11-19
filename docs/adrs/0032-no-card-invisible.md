# No card invisible: unaliased remainder and idle capacity

Every card-holding pod must be visible somewhere. The aggregate and table
views account for the user's selected resources plus the raw underlying
resource of any alias rule whose capacity or demand escaped aliasing: cards on
nodes matching no rule, and Pending demand that stays raw in multi-alias
clusters (ADR 0017). Raw and aliased lines are disjoint by construction —
aliasing renames quantities per node with the first matching rule winning — so
nothing double counts, and workload objects never enter the math: pod requests
are the only accounting input and each pod counts exactly once (ADR 0030's
rollup is labeling only).

Context-level rows now also include clusters that report allocatable capacity
for a resource but run no requesting pods, so idle pools (observed: 624 cards
on unlabeled nodes in one cluster — 77% of its cards — invisible because the
raw resource was unselected) stay in the totals instead of vanishing. This
narrows ADR 0013, which only ever omitted zero-allocatable resources.
