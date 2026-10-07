package apprecordservice

import "sort"

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
	capabilities := workflowLifecycleCapabilities{ReturnTargets: []string{}}
	if !f.CanRead || f.State != "active" {
		return capabilities
	}
	capabilities.Withdraw = f.AllowWithdraw && f.ActorID == f.InitiatorID

	current := false
	for _, task := range f.Tasks {
		if task.InstanceID == f.InstanceID && task.ActorID == f.ActorID && !task.Closed {
			current = true
			break
		}
	}
	targets := make(map[string]bool)
	for _, task := range f.Tasks {
		if task.InstanceID != f.InstanceID {
			continue
		}
		if current || task.ActorID == f.ActorID && task.Closed && task.ClosedAction == "agree" && task.ClosedOutcome == "success" {
			targets[task.NodeID] = true
		}
	}
	for nodeID := range targets {
		capabilities.ReturnTargets = append(capabilities.ReturnTargets, nodeID)
	}
	sort.Strings(capabilities.ReturnTargets)
	return capabilities
}
