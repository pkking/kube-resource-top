# Aggregate queue views: all and series

The queue tab gains the aggregate views the capacity tab has had since
ADRs 0038/0039: a 全部 chip merges every resource's queue and series
chips (910B, 910C) merge one hardware family. The tab defaults to
全部 — the submitter's broadest question is "where is my task
anywhere" — and single-resource chips keep the exact per-resource view
with its verdict banners.

Aggregation happens client-side from the same per-(cluster, resource)
queues: per cluster, member resources' queued workloads concatenate,
re-rank by creation time (the same oldest-first, unknown-last order as
ADR 0037), and gain a 资源 column so each row still names the pool it
targets; demand-ahead accumulates in the merged order. Per-resource
verdicts compress into one colored summary line whose words carry the
full verdict notes as tooltips. The merged rank is a family-wide
approximation — actual scheduling stays per-resource, and the section
summary states the merge explicitly rather than implying one shared
queue.
