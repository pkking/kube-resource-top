# Kubeconfig context identity and selection

Kubeconfig files are parsed independently; a target identity is its absolute file path plus context name, avoiding collisions between identically named contexts. Selections persist in the XDG config file, and the initial selector probes contexts, deselecting unavailable targets.
