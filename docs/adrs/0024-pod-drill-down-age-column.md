# Pod drill-down AGE column

When the scope drill-down reaches the pod level (`len(scope) == 3`), the main table gains a last-column AGE showing each pod's age as a kubectl-style `HumanDuration`, computed at render time from the creationTimestamp captured by the watch (ADR 0001), with a gray `N/A` when the timestamp is unknown. This refines the multi-resource views of ADR 0004: the column is appended in every width variant and the name column reserves extra room so separators stay aligned.

Render-time computation keeps the age honest on every redraw without requiring events; a snapshot-relative age was rejected because it goes stale between watch events, and a refresh tick was rejected because timed polling contradicts the watch-as-state-source model (ADR 0001). The age is display-only: it never participates in aggregation, accounting, or sorting.
