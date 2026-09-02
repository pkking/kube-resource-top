# Single-alias Pending attribution

Pending demand normally retains its raw resource name because it has no assigned Node. When the watched cluster has exactly one alias rule that matches nodes advertising a given source resource, Pending requests for that source resource are attributed to that alias and scaled by its display unit.

If no matching alias, or more than one matching alias, exists in the cluster, Pending demand remains raw. This refines ADR 0005 and ADR 0016 without guessing among multiple hardware variants.
