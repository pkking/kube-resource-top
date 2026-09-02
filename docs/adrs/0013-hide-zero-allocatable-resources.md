# Hide zero-allocatable resources

A resource is omitted from a cluster's detail rows and all drilled-down rows when that cluster reports zero allocatable quantity for it. This prevents Pending or stale requests for an unavailable resource from creating misleading rows. Resources remain visible in clusters with positive allocatable capacity.
