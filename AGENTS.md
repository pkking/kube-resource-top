# Development guide for agents

## Workflow: push directly, no PRs

This project does not use pull requests. Make changes on the default branch
(`master`, tracking `origin/main`) and push directly:

```sh
git add -A
git -c commit.gpgsign=false commit -m "<message>"
git push
```

- Do not open PRs, do not ask for review, do not create feature branches unless
  the work is large enough to warrant isolation.
- Keep commits focused; one logical change per commit.
- Never force-push over shared history without reason.

## Never commit secrets

`.gitignore` already excludes `*.yaml`, `*.yml`, `*.json`, `*kubeconfig`, the
`kube-resource-top` binary, and `.scratch/`. **Kubeconfigs carry cluster
credentials — verify before every push that none are staged:**

```sh
git diff --cached --name-only | grep -iE '\.ya?ml$|kubeconfig|config\.json$|kube-resource-top$|\.scratch' || echo clean
```

## Design and decisions are ADR-first

[`docs/adrs/`](docs/adrs/) is the source of truth for durable decisions
(architecture, integrations, resource accounting, UX, conventions).

**Before designing or implementing any feature:**

1. Read every file in [`docs/adrs/`](docs/adrs/). Preserve the decisions there.
2. If your change touches a durable architectural, integration, accounting, or
   UX decision, write the **next sequential ADR** in `docs/adrs/` *before*
   completing the implementation (or as the first commit of the change).
3. Reference related ADRs by number (e.g. "refines ADR 0005") when an ADR
   builds on or supersedes a prior one.

### ADR format

Each ADR is a short Markdown file named `NNNN-short-kebab-title.md`:

- One `#` title line.
- One or two paragraphs: the decision and the reasoning, including trade-offs
  and why alternatives were rejected. Keep it concise — the file records the
  *decision*, not a design doc.
- Reference prior ADRs by number when relevant.

## Code conventions

- Go. Format with `gofmt`; run `go vet ./...` and `go test ./...` before
  pushing — both must pass.
- Prefer the standard library and already-installed dependencies; do not add a
  new dependency for something a few lines can do.
- Leave behind the smallest runnable check for non-trivial logic: an
  `assert`/`__main__` self-check or one small `test_*.go`/`func Test*`. No
  fixtures or per-function suites unless asked.
- Keep the shortest working diff that is correct on edge cases.

## Verify before pushing

```sh
gofmt -l .
go build ./... && go test ./... && go vet ./...
```
