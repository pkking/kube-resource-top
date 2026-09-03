# Node view scope selection

The node balance view opens in the current drill-down's context and active resource, rather than discarding drill state. It can cycle among that drill scope, all watched nodes and resources, the selected contexts for the active resource, and the selected resource types across selected contexts. Context and resource selections remain the existing persisted multi-selections.

This keeps `b` useful as a contextual drill-down while allowing intentional cross-context and cross-resource comparisons. The node table identifies context and resource whenever either can vary, avoiding ambiguous duplicate node names.
