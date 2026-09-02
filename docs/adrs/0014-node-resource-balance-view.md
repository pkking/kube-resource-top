# Node resource balance view

Pressing `b` toggles a node balance view for the active resource. It lists each node with allocatable capacity, Running Pod requests already assigned to that node, and remaining allocatable capacity. Pending Pods are excluded because they have no node assignment.

The view respects the selected cluster when drilled down and omits nodes with zero allocatable quantity for the active resource. Alias attribution and display-unit scaling are applied consistently to both node capacity and Running requests.
