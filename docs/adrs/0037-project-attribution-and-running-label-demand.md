# Project attribution from runner labels, and label-derived demand while running

Task-to-project attribution reads `actions.github.com/organization` and
`actions.github.com/repository` from the pod's own labels. Observed on
GitHub ARC EphemeralRunner pods and their Liqo ShadowPod reflections: the
labels ride on pods at every phase, so no EphemeralRunner informer is
needed and the ADR 0020/0030 attribution chain is unchanged. The project
identifier is organization/repository when both labels exist, the
repository alone otherwise; demand without the labels aggregates as
neutral unattributed segments.

The `ascend-ci.com/npu-resource-model` + `ascend-ci.com/required-npu-count`
label request (ADR 0015) now also applies to Running pods, still as
max(label, scheduler request) so nothing double counts. Observed on
cn12-001: 144 of 192 ascend-1980 chips run inside ARC runner pods whose
scheduler requests are empty — "scheduler requests only" left 75% of the
pool invisible, which contradicts ADR 0032's no-card-invisible intent.
ARC may pre-provision labeled runners before a job attaches cards; a
labeled runner is counted from provisioning, accepted as the deployment's
own demand statement and bounded by physical reality (144 ≤ 192).

The dashboard renders the split: the chip strip's running and queued
regions subdivide by project color (unattributed keeps the neutral amber
fill and red hatch), each resource shows a donut of the running share
(占用 caliber, matching the 占用 number), and queue rows carry the
project. Colors hash the project name into a fixed ten-color palette, so
a project keeps its color across views and reloads.
