# PR mapping through the GitHub API

With `--github-token FILE` (a token with actions:read, never committed;
the file is gitignored), the board maps running ARC tasks to their pull
requests. The join key is the runner name: GitHub's jobs API returns
`runner_name`, which is exactly the runner pod name the board already
shows (ADR 0044's verified dead end — cluster metadata has no PR — is
bridged from the other side).

A background refresher (2-minute tick, stdlib HTTP) walks the projects
that currently have running tasks: recent workflow runs → in-progress
jobs → runner names, resolving each run's PR by fallback order — the
run's `pull_requests` field, then `/commits/{sha}/pulls`, then a head-
branch PR lookup — and falling back to branch + workflow name when the
API associates no PR (common: scheduled runs, merged branches, the
notoriously sparse pull_requests field). The cache is a plain map with
a read lock; tasks join it at shaping time and carry PR number, PR and
run URLs, branch, and workflow name in the API and the 任务明细 table.
Queued tasks are deliberately not mapped: GitHub assigns runner names
only once a job starts. Rate budget stays far under the 5 000/h
authenticated limit; API failures degrade to no PR data, never to
guessed numbers. The token is read once at startup and never leaves the
process except as an Authorization header.
