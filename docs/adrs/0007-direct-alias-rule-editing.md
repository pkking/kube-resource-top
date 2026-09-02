# Direct alias rule editing

The alias TUI opens on the persisted alias list. `Enter` edits the selected rule and `a` starts a new rule. The editor directly accepts the alias, resource, and comma-separated exact `key=value` label and annotation conditions; it does not require a watched node.

This supersedes the node-metadata authoring flow in ADR 0006. Direct entry supports rules before matching nodes are visible and allows the user to precisely express the existing exact-match configuration contract.
