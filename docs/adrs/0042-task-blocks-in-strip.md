# Task blocks inside the project zones

The capacity strip renders the running and queued regions as one block
per actual workload, sized by its card count, inside the project-colored
zones of ADR 0035/0036: zone order and widths stay exactly the project
view (the zone's tasks sum to the project's demand), so the project
reading is unchanged while every block becomes hoverable — tooltip: task
workload, project, cluster, and card count. Cross-cluster resources
concatenate zones across clusters; each block names its cluster.

Runner fleets produce many single-pod workloads (one ARC runner pod =
one CI job), so each project zone shows its top 8 tasks by demand and
merges the rest into one same-color remainder block with an aggregate
tooltip; queue zones do the same with their pending workloads. Sub-block
separators are 1 px hairlines; identical adjacent colors still read as
one project zone. The API gains the per-resource running task list
(cluster, workload, project, cards), unaggregated — the page owns the
top-8/remainder policy so the visual contract lives in one place. The
donut stays project-level: a per-task donut would be unreadable.
