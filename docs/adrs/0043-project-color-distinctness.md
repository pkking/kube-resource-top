# Project color distinctness: spread palette, zone separators, donut strokes

Ten named-project colors replace the original eight, ordered so that
palette neighbors (which name hashes can produce for adjacent zones)
are always far apart in hue family or lightness — azure-blue, pink,
violet, lime, magenta, cyan, pale rose, mint, pale violet, periwinkle.
The three blue-family entries separate by lightness. The reserved
semantic tones (unattributed amber #e2a23d, queue red #e0564f, table
green #39c97e) stay outside the palette. Hash assignment is unchanged:
a project keeps its color across views and reloads (ADR 0037); the risk
of two adjacent zones hashing to nearby slots is met by the
neighbor-ordering and ten-way spread, not by unstable reassignment —
rank-based coloring would flicker every poll as demand shifts.

Two structural cues reinforce the boundaries: the donut sectors get a
1.5 px dark stroke (the classic sector gap), and each project zone in
the strip starts with a 2 px separator — task blocks inside a zone keep
their 1 px hairlines, so zone edges read twice as strong as task edges.
