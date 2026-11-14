# Workload attribution through the owner chain

Workload-level rows group pods by their top-level controller, resolved through
the controller owner-reference chain (refines ADR 0003's pod accounting; the
demand itself stays pod-based — only pods can request devices, so LWS,
StatefulSet, and other workload pods were always included in the totals). The
watcher adds ReplicaSet, Job, and StatefulSet informers and records each
intermediate object's controller owner as the workload: Deployment pods stop
showing hashed ReplicaSet names, CronJob pods roll up from Jobs, and
LeaderWorkerSet pods roll up from the StatefulSets LWS manages.

The top owner's kind and name come from the intermediate object's owner
reference, so CRD workloads like LeaderWorkerSet resolve without watching the
CRD itself. Resolution happens at snapshot time, not on pod events, so an
owner object arriving after its pods corrects the label on the next snapshot;
owner events re-notify. Owner informers are best-effort: pods and nodes remain
the required sync (ADR 0001), and the owner informers get a bounded grace so a
cluster that denies listing them still renders with one-level attribution.
The EphemeralRunner exception (ADR 0020) is unchanged.
