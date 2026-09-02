# Kubeconfig target discovery after selection

Persisted context selection records both selected targets and the set of targets known when it was saved. On startup, any newly discovered target not in the known set is selected by default; previously known targets retain their saved selection state.

Legacy configs without known targets treat discovered targets absent from the selected list as newly discovered once, then persist the known set. This lets newly added kubeconfig files participate after restart without discarding durable manual selections thereafter.
