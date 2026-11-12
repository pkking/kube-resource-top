# Aggregate capacity totals and occupancy ratio

The ALL CONTEXTS summary at the top of every view now shows, for each selected
resource, the cluster-summed node capacity total and a `req/alloc` occupancy
percentage alongside the existing usage, running, pending, and allocatable
figures. The watcher accumulates node `Status.Capacity` into `snapshot.Total`
next to the allocatable sums in `snapshot.Capacity`, with identical alias
attribution and unit scaling so the two stay comparable; the total follows
allocatable in being summed only at context level (ADR 0003).

The occupancy ratio is request (running + pending) over allocatable, not over
the physical total: allocatable is the scheduling-valid denominator at cluster
scope and matches the table's REQUEST / ALLOCATABLE column and sort mode. A
running-only ratio was rejected because pending demand is part of occupancy
when planning capacity.
