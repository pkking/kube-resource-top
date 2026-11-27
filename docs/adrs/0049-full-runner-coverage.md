# Full runner coverage: wide sweep, matched-run PR resolution

The PR mapping of ADR 0045 sampled at most five runs per project, so on
busy repositories most board runners never appeared in the cache and
their 任务明细 rows fell back to the generic repository Actions link.
Refresh now sweeps widely within a fixed budget: fifty in-progress
runs listed and up to thirty runs' jobs per project on a 3-minute tick
(≤200 calls per refresh, ≤4000/h against the 5000/h quota). PR numbers
resolve only for runs whose jobs matched an actual board runner — the
board's runner set feeds the refresh — under a 25-call sub-budget,
because unmatched runs' PRs are worthless to the page. Job entries
still carry the run URL, job URL, branch, and workflow; a task without
a mapping remains a plain row and never gets a guessed link.
