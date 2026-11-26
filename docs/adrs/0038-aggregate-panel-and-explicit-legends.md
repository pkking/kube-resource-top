# Aggregate occupancy panel, explicit project legends, per-segment tooltips

Refines ADR 0037's rendering. Every project color must identify itself
without a lookup: strip segments and donut sectors carry native tooltips
with the project name and its figures, each resource block gets an
explicit project legend row (color dot + name + running/pending), and
the donut becomes an SVG so each sector tooltips independently. The
categorical legend at the top stays for the unattributed and state
colors only.

The capacity tab gains a leading 汇总 block: a resource-type multiselect
sums the selected types' totals, running, pending, and project splits at
display time. The per-resource accounting is untouched — ADR 0003's
per-resource denominators and ADR 0029's line order still hold inside
each block; the panel only adds figures across the reader's chosen set,
stating that mixed unit scales combine as raw sums at the reader's
direction. Selections persist in the page across polls; newly appearing
resource types join selected.
