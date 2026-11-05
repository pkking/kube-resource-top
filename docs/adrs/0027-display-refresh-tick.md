# Display refresh tick

A 30-second display-only tick re-renders the TUI via the previously unused `refreshMsg`. It performs no API calls and touches no state — the state source remains exclusively the watch (ADR 0001). The tick exists because render-time AGE (ADRs 0024/0025) freezes between events on a quiet cluster: a 3-hour-old pod keeps showing a stale age until some message or key arrives.

The tick distinguishes refreshing the VIEW from polling for STATE, which is what ADR 0001 forbids and what ADR 0024 originally rejected; a state-polling timer remains out of scope. Watcher restarts may stack tick loops (one extra no-op render per restart), accepted as harmless.
