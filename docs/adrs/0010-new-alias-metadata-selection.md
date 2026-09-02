# New alias metadata selection

After selecting a resource for a new alias, the TUI presents distinct multi-select lists of label and annotation `key=value` pairs observed on watched nodes. The user selects conditions without selecting a particular node, then enters the alias name and unit. The selected pairs become the exact-match conditions.

Existing rules remain directly editable. Aggregating observed metadata keeps new-rule authoring keyboard-only while allowing conditions to span node metadata where that is intentional.
