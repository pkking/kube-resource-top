# Kubernetes Watch as the state source

The TUI uses client-go List+Watch informers for Pods and Nodes rather than timed polling. Metrics Server is not treated as a Watch source, so live CPU/memory usage is `N/A` until a reliable streaming metrics source is configured.
