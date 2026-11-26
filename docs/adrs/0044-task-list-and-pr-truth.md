# Task list view, and the honest PR answer

The tasks behind each resource get a browsable home: an expandable
任务明细 table per resource — workload, project, cluster, card count,
sorted by demand — instead of hover-only tooltips on the task blocks.
Project cells for GitHub-identified projects link to their repository's
Actions page (org-only identities link to the organization), the
deepest stable identity the board can offer.

PR numbers do not exist in any Kubernetes metadata this tool watches:
EphemeralRunner objects, runner pods, and workflow pods carry only pool
configuration (patch-id, runnerId, scale-set, runner group) — verified
across labels and annotations cluster-wide. Mapping a running task to
its PR would require the GitHub API (runner name → job → workflow run
→ PR) with actions:read credentials per organization, an external
dependency on user-provided tokens that is out of scope until asked
for. The board therefore links to the Actions page and stops there
rather than guessing.
