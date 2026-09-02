# Node resource alias rules

A node resource alias maps one resource to an alias when all configured label and annotation `key=value` conditions match. Matching Node allocatable and Running Pod requests use the alias; Pending Pods retain their original resource because they have no assigned Node. Potentially overlapping rules for the same resource are rejected rather than selecting an arbitrary alias.
