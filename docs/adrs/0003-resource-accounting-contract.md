# Resource accounting contract

Pod requests and limits use scheduler semantics: `max(sum(app containers), max(init containers)) + overhead`. Only Pending and Running Pods contribute demand; Pending and Running requests remain separate so unscheduled demand is visible. Node allocatable is summed once per context and is only a valid denominator at cluster scope.
