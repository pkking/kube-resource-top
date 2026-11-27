# Workflow boundaries: every task visible in the strip

The per-task blocks of ADR 0042 were capped at eight per zone with a
merged remainder, and their 1 px hairlines were too subtle to read — a
busy zone still looked like one solid bar. The cap now bounds only the
DOM (64 running tasks and 24 queued workloads per zone, merged
remainder beyond that, with the remainder tooltip counting what it
swallows), so each workflow draws its own boundary, its length strictly
proportional to its card count, and a zone's workflow count reads by
counting notches. Separators strengthen to 1.5 px dark lines; zone
edges stay stronger at 2.5 px so the project hierarchy survives —
org color zone > workflow notch. Tooltips still name each workflow with
its PR or branch (ADR 0045), and the 汇总 block stays project-level:
cross-resource task identities are not mergeable into one bar.
