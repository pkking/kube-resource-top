# One organization, one color

The project color key becomes the organization — the first path segment
of org/repo identities, or the bare name otherwise. vllm-project,
vllm-project/vllm-ascend, and any other vllm-project/* identity now
share one color in every tab; the same string still hashes to the same
color everywhere (ADR 0037's stability promise holds, now across
identity variants too — the org-level namespace identities of ADR 0041
match their repositories' color instead of looking unrelated between
tabs). Refines ADR 0043's palette spread; unattributed demand keeps the
reserved amber.

Trade-off: repositories within one organization share the org's color
and are no longer distinguishable by hue alone. The distinction moves
where it belongs: the zone tooltips, the task-block hairlines, and the
项目/任务明细 tables name the exact repository, and the board's primary
color question — which organization holds the cards — reads cleaner.
