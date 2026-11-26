# Occupancy time series: local JSONL recording and history view

The web mode can record occupancy as a time series with `--web-record
DIR`. The recorder samples the same in-memory watch snapshots the page
serves, on a timer (`--web-record-interval`, default 1 m): it re-reads
state that Kubernetes Watch already delivered, in ADR 0027's sense — no
cluster is polled (ADR 0001 is untouched), and recording is opt-in
because a long-running process should not write disk unless asked.

Storage is stdlib JSONL, one line per (cluster, resource) sample: unix
time, cluster name, resource, total/allocatable/running/pending, queued
workload count, and the per-cluster project split (ADR 0037) so history
can stack by project. One file per UTC day (`history-YYYY-MM-DD.jsonl`),
the previous day is gzipped on rotation, and files older than
`--web-record-keep` days (default 30) are deleted. Raw lines cost ~20 MB
per day at the default cadence with 12 clusters; gzip brings a day to a
few MB, so no external TSDB dependency is introduced (repo convention:
stdlib only).

回看 reads the files server-side: `/api/history.json?res=NAME&cluster=…&
hours=…` filters to one resource and one cluster or all, keeps the last
sample per (cluster, bucket) and sums across clusters, and downsamples
to at most 288 bucket-end points. The history tab stacks running demand
by project color (solid areas, the capacity strip's language), queues as
hatched area on top, with a dashed allocatable reference and a hover
readout. Bucket-end values state occupancy at the bucket boundary;
sub-bucket spikes are not visible, which the view presents as-is.
