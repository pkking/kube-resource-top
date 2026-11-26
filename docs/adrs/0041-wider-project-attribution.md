# Wider project attribution: workflow pods, scale sets, and project namespaces

Unattributed demand was dominated by three observed classes, and each gets
an evidence-based rule (extends ADR 0037; anything still unmatched stays
未标记 rather than guessed):

1. ARC workflow pods carry `runner-pod=<runner-pod-name>` and their own
   name embeds the scale set (`<scale-set>-runner-<id>-workflow`), but no
   GitHub identity labels. At snapshot time they inherit the project of
   the referenced runner pod in the same namespace; if that pod is gone,
   they fall back to a scale-set → project map learned from runner pods
   that carry both `actions.github.com/scale-set-name` and the repository
   identity. On cn12-001 this covers 33 of 36 unattributed NPU pods.
2. Shared-pool runners (hk-001's ascend-gha-runners) carry an
   organization but no repository, because shared runners serve any repo:
   they attribute to the organization alone.
3. A namespace that exactly equals an observed repository name or GitHub
   organization attributes its pods to that identity (vllm-project → the
   org, triton-ascend → triton-lang/triton-ascend, nv-action → the org,
   the LWS StatefulSets in vllm-project included). Pool or system
   namespaces (arc-fedsched-*, argo, ascend-gha-runners-*) match nothing
   and stay unattributed; argo/karmada dispatch pods carry no identity at
   all and are the honest remainder.

All three resolutions happen at snapshot time from watched pods only —
no new informers. The scale-set, repository-name, and organization maps
persist for the watcher's lifetime: they are learned configuration, and
stale entries can only attribute pods that genuinely belong to that
identity's infrastructure.
