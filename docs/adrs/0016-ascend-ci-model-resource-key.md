# Ascend CI model resource key

A Pending Pod's `ascend-ci.com/npu-resource-model` value is an Ascend model suffix. If it is not already a qualified resource name, kube-resource-top maps it to `huawei.com/<model>` before adding the label-derived request. For example, `ascend-1980` becomes `huawei.com/ascend-1980`.

This keeps label-derived Pending demand under the same raw Kubernetes resource key as Node allocatable capacity. As established in ADR 0005, Pending demand is not alias- or unit-scaled because it has no assigned node.
