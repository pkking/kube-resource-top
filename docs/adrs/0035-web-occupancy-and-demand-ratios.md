# Web capacity line: occupancy and demand are two ratios

The web capacity line (ADR 0034) labeled the ADR 0028 planning ratio —
request (running + pending) over allocatable — as 占用 (occupancy). A live
reader computed 121 running over 184 allocatable, expected 66%, and read
101%: the number was right per ADR 0028, but the noun promised current
usage. Pending demand does belong in a planning ratio and still does
(ADR 0028's reasoning is unchanged), while the chip strip's amber fill is
running-only — the number beside the fill must measure what the fill
measures.

The line therefore shows two named ratios: 占用 (running/allocatable,
matches the amber fill) and 需求 ((running + pending)/allocatable, ADR
0028's ratio, matches the red demand line and drives the 超配 pill). The
line keeps ADR 0029's order with 占用 inserted first:
总量 → 占用 → 需求 → 运行中 → 排队 → 可分配 → 超配. The strip tooltips
state each ratio's formula so the arithmetic is self-explaining.
