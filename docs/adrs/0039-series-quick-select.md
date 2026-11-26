# Series quick-select in the aggregate panel

The 汇总 resource multiselect (ADR 0038) gains series buttons: one click
selects or clears every resource of one hardware series, so 910B toggles
910B1/910B3/910B4 and 910C toggles the 910C pool at once. A series key is
derived per resource name with the heuristic "name up to the last
letter-or-dash that directly precedes a terminal variant beginning with
a digit" — 910B1 → 910B, 910C560T → 910C, huawei.com/ascend-310 →
huawei.com/ascend. The rule can over-group exotic names (gpu-h100 →
gpu-h), which is accepted: a series button only flips its member chips,
and the per-resource chips remain the exact control. Series with fewer
than two present resources render no button; clicking a series toggles
all members on if any is off, off when all are on, and the button's
tooltip lists its members.
