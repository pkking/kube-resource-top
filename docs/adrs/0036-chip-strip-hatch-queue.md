# Chip strip encodes the queue as a hatch band behind running work

The ADR 0035 demand ratio shipped with a companion graphic: a separate
3 px red bar under the chip strip, same x-scale, clamped at 100 %. Its
first live readers could not tell what it measured — an unlabeled sub-bar
invites guesses, with no pending work it exactly duplicated the amber
fill above it, and when demand exceeded capacity the clamp hid the very
overflow it existed to show. Refines ADR 0035; the two named ratios and
the numbers row are unchanged.

The strip is now a single meter: solid amber from the left measures
running work (占用), a red diagonal-hatch band appended at the amber edge
measures queued demand (需求), and outlined cells beyond it stay visibly
idle. The hatch reaching the right edge means queued demand has filled
all allocatable capacity, and the 超配 pill carries the overshoot
amount. No second bar exists to be misread, and the hatch shares the
strip's cell texture so it reads as part of the same inventory instead
of a separate chart.
