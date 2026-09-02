# EphemeralRunner Pod attribution

`EphemeralRunner` is a controller owner of a core Kubernetes Pod, not a separately collected workload. For drill-down display, Pods controlled by `EphemeralRunner` are attributed as `Pod/<pod-name>` rather than exposing the controller name. Their requests remain counted once as Pod requests.
