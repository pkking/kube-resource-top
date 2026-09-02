# Pending NPU label requests

For Pending Pods, `ascend-ci.com/npu-resource-model` and a positive integer `ascend-ci.com/required-npu-count` express an additional requested extended resource. The pending request uses the model label as its resource name and the count label as its quantity.

If the Pod already has a scheduler resource request under that same resource name, the larger quantity is retained rather than double-counting the same request. Running Pods continue to use scheduler requests only.
