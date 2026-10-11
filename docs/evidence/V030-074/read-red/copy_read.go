package apptemplates

import (
	"context"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

var ErrCopySourceChanged = errors.New("copy source changed")

type CopySource struct {
	AppID string `json:"appId"`
	Kind  string `json:"kind"`
	ID    string `json:"id"`
}
type CopyTarget struct {
	AppID       *string `json:"appId"`
	DirectoryID *string `json:"directoryId"`
	Name        string  `json:"name"`
}
type CopyPlanInput struct {
	Source CopySource `json:"source"`
	Target CopyTarget `json:"target"`
}
type CopyPlan struct {
	SourceFingerprint      string     `json:"sourceFingerprint"`
	TargetStructureVersion int64      `json:"targetStructureVersion"`
	TargetPolicyRevision   int64      `json:"targetPolicyRevision"`
	Manifest               Manifest   `json:"manifest"`
	ExternalReferences     References `json:"externalReferences"`
}
type CopyPreflightInput struct {
	Source                         CopySource         `json:"source"`
	Target                         CopyTarget         `json:"target"`
	ExpectedSourceFingerprint      string             `json:"expectedSourceFingerprint"`
	ExpectedTargetStructureVersion int64              `json:"expectedTargetStructureVersion"`
	ExpectedTargetPolicyRevision   int64              `json:"expectedTargetPolicyRevision"`
	Bindings                       []Binding          `json:"bindings"`
	GroupMappings                  []CopyGroupMapping `json:"groupMappings"`
}
type CopyPreflight struct {
	Valid          bool     `json:"valid"`
	Counts         Counts   `json:"counts"`
	MappedManifest Manifest `json:"mappedManifest"`
}

func (s *Service) PlanCopy(ctx context.Context, p session.Principal, in CopyPlanInput) (CopyPlan, error) {
	return CopyPlan{}, ErrInvalid
}
func (s *Service) PreflightCopy(ctx context.Context, p session.Principal, in CopyPreflightInput) (CopyPreflight, error) {
	return CopyPreflight{}, ErrInvalid
}
