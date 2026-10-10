package apprecordservice

type workflowRoundIdentity struct {
	AppID, TableID, RecordID, FlowID, InstanceID string
}
type workflowRoundFacts struct {
	Target, Latest              workflowRoundIdentity
	ActorID, InitiatorID, State string
	CanRead, CanEdit            bool
	DefinitionApprovers         []string
}
type workflowRoundCapabilities struct{ Rework, Resubmit, Review bool }

func workflowRoundPolicy(f workflowRoundFacts) workflowRoundCapabilities {
	return workflowRoundCapabilities{}
}
