package apprecordservice

type workflowLifecycleTask struct {
	InstanceID, ActorID, NodeID, ClosedAction, ClosedOutcome string
	Closed                                                   bool
}
type workflowLifecycleFacts struct {
	ActorID, InitiatorID, InstanceID, State string
	AllowWithdraw, CanRead                  bool
	Tasks                                   []workflowLifecycleTask
}
type workflowLifecycleCapabilities struct {
	Withdraw      bool
	ReturnTargets []string
}

func workflowLifecyclePolicy(f workflowLifecycleFacts) workflowLifecycleCapabilities {
	return workflowLifecycleCapabilities{ReturnTargets: []string{}}
}
