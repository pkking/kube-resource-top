# New alias metadata scoped to resource nodes

When adding an alias, after choosing its source resource the label and annotation pickers show only metadata observed on nodes with allocatable capacity for that resource. This refines ADR 0010 so conditions are discoverable from the candidate hardware rather than unrelated nodes; existing-rule editing remains cluster-wide to preserve direct editing of exceptional values.
