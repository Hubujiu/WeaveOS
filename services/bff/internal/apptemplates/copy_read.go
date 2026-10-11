package apptemplates

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
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

func normalizeCopyPlan(in CopyPlanInput) (CopyPlanInput, error) {
	if !validID(in.Source.AppID) || !validID(in.Source.ID) {
		return in, ErrInvalid
	}
	switch in.Source.Kind {
	case "application":
		if in.Source.ID != in.Source.AppID || in.Target.AppID != nil || in.Target.DirectoryID != nil {
			return in, ErrInvalid
		}
	case "directory", "table", "form":
		if in.Target.AppID == nil || !validID(*in.Target.AppID) {
			return in, ErrInvalid
		}
	default:
		return in, ErrInvalid
	}
	if in.Target.DirectoryID != nil && !validID(*in.Target.DirectoryID) {
		return in, ErrInvalid
	}
	var ok bool
	in.Target.Name, ok = name(in.Target.Name)
	if !ok {
		return in, ErrInvalid
	}
	return in, nil
}
func sourceFingerprint(m Manifest) (string, error) {
	raw, e := json.Marshal(m)
	if e != nil {
		return "", e
	}
	hash := sha256.Sum256(append([]byte("weaveos.structure-copy.source.v1\n"), raw...))
	return hex.EncodeToString(hash[:]), nil
}
func (s *Service) beginCopyPlan(ctx context.Context, p session.Principal, in CopyPlanInput) (pgx.Tx, CopyPlan, error) {
	if s == nil || s.Pool == nil {
		return nil, CopyPlan{}, session.ErrUnavailable
	}
	in, e := normalizeCopyPlan(in)
	if e != nil {
		return nil, CopyPlan{}, e
	}
	target := ""
	if in.Target.AppID != nil {
		target = *in.Target.AppID
	}
	tx, app, version, policy, e := (&applications.Application{Pool: s.Pool}).BeginStructureCopyRead(ctx, p, in.Source.AppID, target, in.Target.DirectoryID)
	if e != nil {
		return nil, CopyPlan{}, e
	}
	fail := func(err error) (pgx.Tx, CopyPlan, error) {
		_ = tx.Rollback(context.Background())
		return nil, CopyPlan{}, err
	}
	full, e := exportManifestInTx(ctx, tx, app)
	if e != nil {
		return fail(e)
	}
	m, e := SelectCopyManifest(full, in.Source.Kind, in.Source.ID)
	if e != nil {
		return fail(e)
	}
	fingerprint, e := sourceFingerprint(m)
	if e != nil {
		return fail(e)
	}
	refs, e := ExternalReferences(m)
	if e != nil {
		return fail(e)
	}
	return tx, CopyPlan{SourceFingerprint: fingerprint, TargetStructureVersion: version, TargetPolicyRevision: policy, Manifest: m, ExternalReferences: refs}, nil
}
func (s *Service) PlanCopy(ctx context.Context, p session.Principal, in CopyPlanInput) (CopyPlan, error) {
	tx, plan, e := s.beginCopyPlan(ctx, p, in)
	if e != nil {
		return CopyPlan{}, e
	}
	defer tx.Rollback(context.Background())
	if e = tx.Commit(ctx); e != nil {
		return CopyPlan{}, e
	}
	return plan, nil
}
func (s *Service) PreflightCopy(ctx context.Context, p session.Principal, in CopyPreflightInput) (CopyPreflight, error) {
	if len(in.ExpectedSourceFingerprint) != 64 || in.ExpectedTargetStructureVersion < 0 || in.ExpectedTargetPolicyRevision < 0 || in.ExpectedTargetStructureVersion > 9007199254740991 || in.ExpectedTargetPolicyRevision > 9007199254740991 || in.Bindings == nil || in.GroupMappings == nil {
		return CopyPreflight{}, ErrInvalid
	}
	for _, c := range in.ExpectedSourceFingerprint {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return CopyPreflight{}, ErrInvalid
		}
	}
	tx, plan, e := s.beginCopyPlan(ctx, p, CopyPlanInput{Source: in.Source, Target: in.Target})
	if e != nil {
		return CopyPreflight{}, e
	}
	defer tx.Rollback(context.Background())
	if in.ExpectedSourceFingerprint != plan.SourceFingerprint {
		return CopyPreflight{}, ErrCopySourceChanged
	}
	if in.ExpectedTargetStructureVersion != plan.TargetStructureVersion {
		return CopyPreflight{}, &appstructure.Error{Code: "APPLICATION_STRUCTURE_CONFLICT", Data: map[string]int64{"currentStructureVersion": plan.TargetStructureVersion}}
	}
	if in.ExpectedTargetPolicyRevision != plan.TargetPolicyRevision {
		return CopyPreflight{}, applications.ErrPolicyConflict
	}
	m, e := MapExternalReferences(plan.Manifest, in.Bindings)
	if e != nil {
		return CopyPreflight{}, e
	}
	m, e = MapCopyGroups(m, in.GroupMappings)
	if e != nil {
		return CopyPreflight{}, e
	}
	refs, e := ExternalReferences(m)
	if e != nil {
		return CopyPreflight{}, e
	}
	sources := appstructure.CurrentSources{}
	if len(refs.UserIDs) > 0 {
		if e = sources.Validate(ctx, tx, "member", refs.UserIDs); e != nil {
			return CopyPreflight{}, e
		}
	}
	if len(refs.DepartmentIDs) > 0 {
		if e = sources.Validate(ctx, tx, "department", refs.DepartmentIDs); e != nil {
			return CopyPreflight{}, e
		}
	}
	counts := Counts{Directories: len(m.Directories), Tables: len(m.Tables), Forms: len(m.Forms), Workflows: len(m.Workflows), PermissionGroups: len(m.PermissionGroups)}
	for _, table := range m.Tables {
		counts.Fields += len(table.Fields)
	}
	if e = tx.Commit(ctx); e != nil {
		return CopyPreflight{}, e
	}
	return CopyPreflight{Valid: true, Counts: counts, MappedManifest: m}, nil
}
