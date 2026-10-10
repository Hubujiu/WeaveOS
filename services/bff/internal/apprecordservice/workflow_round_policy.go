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
	out := workflowRoundCapabilities{}
	if !f.CanRead || f.ActorID == "" || f.InitiatorID == "" || f.Target != f.Latest {
		return out
	}
	for _, id := range []string{f.Target.AppID, f.Target.TableID, f.Target.RecordID, f.Target.FlowID, f.Target.InstanceID} {
		if id == "" {
			return out
		}
	}
	switch f.State {
	case "rejected", "withdrawn":
		out.Resubmit = f.ActorID == f.InitiatorID
		out.Rework = out.Resubmit && f.CanEdit
	case "completed":
	default:
		return out
	}
	for _, actor := range f.DefinitionApprovers {
		if actor == f.ActorID {
			out.Review = true
			break
		}
	}
	return out
}
