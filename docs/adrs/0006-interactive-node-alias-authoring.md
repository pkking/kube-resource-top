# Interactive node alias authoring

Node alias rules are authored from the TUI. The user selects a discovered resource, an alias name, a watched node, then any exact label and annotation pairs from that node. Saving validates the complete rule set, persists it in the existing `aliases` config section, and restarts the selected context watches so accounting immediately reflects the rule.

The editor deliberately uses values from one node as conditions; it does not provide free-form selector entry. This keeps the persisted exact-match contract visible and avoids creating rules for metadata that has not been observed.
