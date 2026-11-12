# Aggregate line order and usage removal

The ALL CONTEXTS summary line per resource reads total → req/alloc → running
→ pending → allocatable (refines ADR 0028). The total and its occupancy
percentage lead because they answer "how big and how full" at a glance; the
running/pending request breakdown and allocatable follow.

The usage/request pair is dropped from this line: usage is N/A until a
streaming metrics source exists, it is unrelated to the allocation ratio, and
per-row usage stays visible in the table's USAGE / REQUEST column. Total
request remains derivable as running + pending.
