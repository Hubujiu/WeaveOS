package apprecordservice

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appdrafts"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecords"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appschema"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
)

type controlledDML struct{ implementation appstructure.RecordDML }

func validWriteLimits(l appschema.Limits) bool {
	return l.LockTimeout >= time.Millisecond && l.StatementTimeout >= time.Millisecond
}

func (d controlledDML) Insert(ctx context.Context, tx pgx.Tx, t apprecords.Table, in apprecords.Create, ids []string) (apprecords.StoredHeader, error) {
	h, e := d.implementation.Insert(ctx, tx, appstructure.RecordTable(t), appstructure.RecordCreate(in), ids)
	return apprecords.StoredHeader(h), e
}
func (d controlledDML) LockHeader(ctx context.Context, tx pgx.Tx, t apprecords.Table, id string) (apprecords.StoredHeader, error) {
	h, e := d.implementation.LockHeader(ctx, tx, appstructure.RecordTable(t), id)
	return apprecords.StoredHeader(h), e
}
func (d controlledDML) UpdateCAS(ctx context.Context, tx pgx.Tx, t apprecords.Table, in apprecords.Edit, ids []string) (apprecords.StoredHeader, error) {
	h, e := d.implementation.UpdateCAS(ctx, tx, appstructure.RecordTable(t), appstructure.RecordEdit(in), ids)
	return apprecords.StoredHeader(h), e
}

type recordAuthorization struct{ policy appaccess.Policy }

func (a recordAuthorization) Check(_ context.Context, _ pgx.Tx, action, owner string, ids []string) error {
	switch action {
	case "data.create":
		if a.policy.CanCreate(ids) {
			return nil
		}
	case "data.edit":
		if a.policy.CanEdit(owner, ids) {
			return nil
		}
	}
	return applications.ErrDenied
}

type recordAudit struct{ port appstructure.RecordAudit }

func (a recordAudit) Append(ctx context.Context, tx pgx.Tx, result apprecords.MutationResult, kind string, ids []string) error {
	return a.port.Append(ctx, tx, appstructure.RecordMutationResult(result), kind, ids)
}

func policyFor(facts applications.RecordContext) (appaccess.Policy, bool) {
	p := appaccess.Policy{ActorID: facts.Actor.ID, AppID: facts.App.ID, OwnerID: facts.App.OwnerUserID, ViewID: facts.ViewID, ResourceExists: appfields.ValidID(facts.TableID), BootstrapAdmin: facts.Actor.BootstrapAdmin}
	menu := p.BootstrapAdmin || p.ActorID == p.OwnerID
	for _, g := range facts.Grants {
		if g.ResourceKind != "form" || g.ResourceID != facts.ViewID {
			continue
		}
		if g.Action == "menu.enter" {
			menu = true
			continue
		}
		var scope appaccess.Scope
		switch g.RowScope {
		case "all":
			scope = appaccess.All
		case "own":
			scope = appaccess.Own
		default:
			return p, false
		}
		p.Grants = append(p.Grants, appaccess.Grant{AppID: facts.App.ID, ViewID: facts.ViewID, Action: appaccess.Action(g.Action), Scope: scope, Fields: g.Fields})
	}
	return p, menu
}

func normalizeRecordValues(raw map[string]any, fields []appfields.Field, requireComplete bool) (map[string]any, error) {
	if raw == nil {
		return nil, apprecords.ErrInvalid
	}
	definitions := map[string]appfields.Field{}
	for _, f := range fields {
		definitions[f.ID] = f
	}
	ids := make([]string, 0, len(raw))
	for id := range raw {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	values := make(map[string]any, len(raw))
	for _, id := range ids {
		f, ok := definitions[id]
		if !ok {
			return nil, fmt.Errorf("unknown edit field %s: %w", id, apprecords.ErrInvalid)
		}
		encoded, e := json.Marshal(raw[id])
		if e != nil {
			return nil, apprecords.ErrInvalid
		}
		canonical, e := appfields.NormalizeValue(f, encoded)
		if e != nil {
			return nil, e
		}
		var value any
		if e = json.Unmarshal(canonical, &value); e != nil {
			return nil, e
		}
		values[id] = value
	}
	for _, f := range fields {
		if !requireComplete {
			continue
		}
		if f.Required {
			if _, ok := values[f.ID]; !ok && (len(f.Default) == 0 || string(f.Default) == "null") {
				return nil, apprecords.ErrInvalid
			}
		}
	}
	return values, nil
}

func fieldsInContext(facts applications.RecordContext) ([]appfields.Field, []string, error) {
	var fields []appfields.Field
	if json.Unmarshal(facts.Fields, &fields) != nil {
		return nil, nil, ErrUnavailable
	}
	ids := make([]string, 0, len(fields))
	seen := map[string]bool{}
	for _, f := range fields {
		if !appfields.ValidID(f.ID) || seen[f.ID] {
			return nil, nil, ErrUnavailable
		}
		seen[f.ID] = true
		ids = append(ids, f.ID)
	}
	return fields, ids, nil
}

// BeginRecordWrite holds personnel.lock_query_revisions before this check.
// All auth/users, department and membership source writers take that lock
// before their source triggers, so active source facts cannot change until
// this record transaction commits.
func validateNewReferences(ctx context.Context, tx pgx.Tx, values map[string]any, fields []appfields.Field, applyDefaults bool) error {
	for _, f := range fields {
		if f.Kind != "member" && f.Kind != "department" {
			continue
		}
		value, supplied := values[f.ID]
		if !supplied && applyDefaults && len(f.Default) > 0 && string(f.Default) != "null" {
			if err := json.Unmarshal(f.Default, &value); err != nil {
				return ErrUnavailable
			}
			supplied = true
		}
		if supplied && value != nil {
			id, valid := value.(string)
			if !valid || !appfields.ValidID(id) {
				return applications.ErrResourceInvalid
			}
			source := "applications.member_sources"
			if f.Kind == "department" {
				source = "applications.department_sources"
			}
			var active bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM "+source+" WHERE id=$1 AND status='active')", id).Scan(&active); err != nil {
				return ErrUnavailable
			}
			if !active {
				return applications.ErrResourceInvalid
			}
		}
	}
	return nil
}

func (s *Service) Create(ctx context.Context, principal session.Principal, req CreateRequest, metadata applications.Metadata) (MutationResult, error) {
	var result MutationResult
	if s == nil || s.Pool == nil || !validWriteLimits(s.Limits) || !appfields.ValidID(req.AppID) || !appfields.ValidID(req.ViewID) || !appfields.ValidID(req.OperationID) || req.Values == nil || req.ExpectedSchemaVersion < 0 || metadata.RequestID == "" {
		return result, apprecords.ErrInvalid
	}
	canonical, _ := json.Marshal(struct {
		Kind, AppID, ViewID string
		Body                CreateRequest
	}{"record.create", req.AppID, req.ViewID, req})
	fingerprint := sha256.Sum256(canonical)
	var policy appaccess.Policy
	var fields []appfields.Field
	options := applications.RecordWriteOptions{LockTimeout: s.Limits.LockTimeout, StatementTimeout: s.Limits.StatementTimeout, OperationID: req.OperationID, Kind: "record.create", Fingerprint: fingerprint,
		Authorize: func(c context.Context, tx pgx.Tx, facts applications.RecordContext) error {
			var menu bool
			policy, menu = policyFor(facts)
			if !menu {
				return applications.ErrDenied
			}
			var ids []string
			var err error
			fields, ids, err = fieldsInContext(facts)
			if err != nil {
				return err
			}
			_ = ids
			selected := make([]string, 0, len(req.Values))
			for id := range req.Values {
				selected = append(selected, id)
			}
			if !policy.CanCreate(selected) {
				return applications.ErrDenied
			}
			if !facts.SchemaReady {
				return &appstructure.Error{Code: "APPLICATION_SCHEMA_NOT_READY"}
			}
			return validateNewReferences(c, tx, req.Values, fields, true)
		},
	}
	write, replayed, err := s.beginRecordMutation(ctx, principal, req.AppID, req.ViewID, req.QueryVersion, options)
	if err != nil {
		return result, err
	}
	defer write.Rollback(context.Background())
	if replayed != nil {
		if err = write.Commit(ctx); err != nil {
			return result, err
		}
		if json.Unmarshal(replayed.Data, &result) != nil {
			return MutationResult{}, ErrUnavailable
		}
		return result, nil
	}
	if err = write.Claim(ctx, req.OperationID, "record.create", fingerprint); err != nil {
		return result, err
	}
	facts := write.Context()
	if req.ExpectedSchemaVersion != facts.SchemaVersion {
		return result, &appstructure.Error{Code: "APPLICATION_SCHEMA_CONFLICT", Data: map[string]int64{"currentSchemaVersion": facts.SchemaVersion}}
	}
	values, err := normalizeRecordValues(req.Values, fields, true)
	if err != nil {
		return result, err
	}
	var id string
	if err = write.Tx().QueryRow(ctx, "SELECT gen_random_uuid()::text").Scan(&id); err != nil {
		return result, err
	}
	writer := apprecords.Writer{Gate: appstructure.RecordGate{AppID: req.AppID, TableID: facts.TableID, ViewID: req.ViewID}, Authorization: recordAuthorization{policy: policy}, Fence: appstructure.RecordFence{AppID: req.AppID, TableID: facts.TableID}, Audit: recordAudit{port: appstructure.RecordAudit{Context: facts, OperationID: req.OperationID, Metadata: metadata}}, DML: controlledDML{}}
	_, active, err := fieldsInContext(facts)
	if err != nil {
		return result, err
	}
	stored, err := writer.CreateInTx(ctx, write.Tx(), apprecords.Table{AppID: req.AppID, TableID: facts.TableID, ViewID: req.ViewID, Namespace: "appdata", SchemaVersion: facts.SchemaVersion, Ready: facts.SchemaReady, ActiveFieldIDs: active}, apprecords.Create{OperationID: req.OperationID, ID: id, ActorID: facts.Actor.ID, ExpectedSchemaVersion: req.ExpectedSchemaVersion, Values: values})
	if err != nil {
		return result, err
	}
	if req.DraftRef != nil {
		store := appdrafts.Store{Relation: pgx.Identifier{"applications", "record_drafts"}}
		if err = store.ConsumeInTx(ctx, write.Tx(), draftAccess(facts, policy, fields, nil, ""), req.DraftRef.ID, req.DraftRef.DraftVersion, nil, nil); err != nil {
			return MutationResult{}, err
		}
	}
	result = MutationResult{OperationID: stored.OperationID, ID: stored.ID, RecordVersion: stored.RecordVersion, SchemaVersion: stored.SchemaVersion, CreatedAt: stored.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: stored.UpdatedAt.UTC().Format(time.RFC3339Nano)}
	raw, _ := json.Marshal(result)
	if err = write.Complete(ctx, req.OperationID, applications.Result{Status: 201, Location: "", Data: raw}); err != nil {
		return MutationResult{}, err
	}
	if err = write.Commit(ctx); err != nil {
		return MutationResult{}, err
	}
	return result, nil
}

var _ apprecords.TypedDML = controlledDML{}
var _ apprecords.Authorization = recordAuthorization{}
var _ apprecords.Audit = recordAudit{}

func draftAccess(facts applications.RecordContext, policy appaccess.Policy, fields []appfields.Field, base *int64, targetOwner string) appdrafts.Access {
	known := map[string]appfields.Field{}
	for _, f := range fields {
		known[f.ID] = f
	}
	allowed := policy.CanCreate(nil)
	if targetOwner != "" {
		allowed = policy.CanEdit(targetOwner, nil)
	}
	return appdrafts.Access{ActorID: facts.Actor.ID, AppID: facts.App.ID, TableID: facts.TableID, ViewID: facts.ViewID,
		ResourceAllowed: allowed, CurrentSchemaVersion: facts.SchemaVersion, CurrentBaseRecordVersion: base,
		Field: func(id string) appdrafts.FieldStatus {
			f, ok := known[id]
			if !ok {
				return appdrafts.FieldStatus{}
			}
			maxRunes := 1 << 20
			maxItems := 1000
			var config struct {
				MaxLength *int  `json:"maxLength"`
				Options   []any `json:"options"`
			}
			_ = json.Unmarshal(f.Config, &config)
			if config.MaxLength != nil && *config.MaxLength > 0 {
				maxRunes = *config.MaxLength
			}
			if len(config.Options) > 0 {
				maxItems = len(config.Options)
			}
			writable := policy.CanCreate([]string{id})
			if targetOwner != "" {
				writable = policy.CanEdit(targetOwner, []string{id})
			}
			return appdrafts.FieldStatus{Exists: true, Writable: writable, Kind: f.Kind, MaxRunes: maxRunes, MaxItems: maxItems}
		},
	}
}

func (s *Service) CreateDraft(ctx context.Context, principal session.Principal, req DraftCreateRequest, metadata applications.Metadata) (appdrafts.Draft, error) {
	var empty appdrafts.Draft
	if s == nil || s.Pool == nil || !validWriteLimits(s.Limits) || !appfields.ValidID(req.AppID) || !appfields.ValidID(req.ViewID) || !appfields.ValidID(req.OperationID) || req.SchemaVersion < 1 || req.Values == nil || metadata.RequestID == "" {
		return empty, appdrafts.ErrInvalid
	}
	canonical, _ := json.Marshal(struct {
		Kind, AppID, ViewID string
		Body                DraftCreateRequest
	}{"draft.create", req.AppID, req.ViewID, req})
	fingerprint := sha256.Sum256(canonical)
	var policy appaccess.Policy
	var fields []appfields.Field
	options := applications.RecordWriteOptions{LockTimeout: s.Limits.LockTimeout, StatementTimeout: s.Limits.StatementTimeout, OperationID: req.OperationID, Kind: "draft.create", Fingerprint: fingerprint,
		Authorize: func(_ context.Context, _ pgx.Tx, facts applications.RecordContext) error {
			var menu bool
			policy, menu = policyFor(facts)
			if !menu {
				return applications.ErrDenied
			}
			var err error
			fields, _, err = fieldsInContext(facts)
			if err != nil {
				return err
			}
			if req.TargetRecordID == nil && !policy.CanCreate(nil) {
				return applications.ErrDenied
			}
			if req.TargetRecordID != nil && policy.VisibleScope() == appaccess.None && !policy.CanEdit(facts.Actor.ID, nil) {
				return applications.ErrDenied
			}
			return nil
		},
	}
	write, err := (&applications.Application{Pool: s.Pool}).BeginRecordWrite(ctx, principal, req.AppID, req.ViewID, options)
	if err != nil {
		return empty, err
	}
	defer write.Rollback(context.Background())
	replayed, err := write.Replay(ctx, req.OperationID, "draft.create", fingerprint)
	if err != nil {
		return empty, err
	}
	if replayed != nil {
		if err = write.Commit(ctx); err != nil {
			return empty, err
		}
		var minimum struct {
			ID           string `json:"id"`
			DraftVersion int64  `json:"draftVersion"`
		}
		if json.Unmarshal(replayed.Data, &minimum) != nil {
			return empty, ErrUnavailable
		}
		return appdrafts.Draft{ID: minimum.ID, DraftVersion: minimum.DraftVersion}, nil
	}
	if err = write.Claim(ctx, req.OperationID, "draft.create", fingerprint); err != nil {
		return empty, err
	}
	facts := write.Context()
	if req.SchemaVersion != facts.SchemaVersion {
		return empty, appdrafts.ErrBaseConflict
	}
	if err = (appstructure.RecordGate{AppID: req.AppID, TableID: facts.TableID, ViewID: req.ViewID}).LockTable(ctx, write.Tx(), facts.TableID, facts.SchemaVersion); err != nil {
		return empty, err
	}
	var base *int64
	var targetOwner string
	if req.TargetRecordID != nil {
		if !appfields.ValidID(*req.TargetRecordID) || req.BaseRecordVersion == nil {
			return empty, appdrafts.ErrInvalid
		}
		relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(facts.TableID, "-", "")}.Sanitize()
		var version int64
		if err = write.Tx().QueryRow(ctx, "SELECT record_version,created_by::text FROM "+relation+" WHERE id=$1", *req.TargetRecordID).Scan(&version, &targetOwner); err != nil {
			return empty, err
		}
		base = &version
		if version != *req.BaseRecordVersion {
			return empty, appdrafts.ErrBaseConflict
		}
	}
	access := draftAccess(facts, policy, fields, base, targetOwner)
	var id string
	if err = write.Tx().QueryRow(ctx, "SELECT gen_random_uuid()::text").Scan(&id); err != nil {
		return empty, err
	}
	store := appdrafts.Store{Relation: pgx.Identifier{"applications", "record_drafts"}}
	draft, err := store.CreateInTx(ctx, write.Tx(), access, appdrafts.Create{ID: id, TargetRecordID: req.TargetRecordID, SchemaVersion: req.SchemaVersion, BaseRecordVersion: req.BaseRecordVersion, Values: req.Values})
	if err != nil {
		return empty, err
	}
	raw, _ := json.Marshal(map[string]any{"operationId": req.OperationID, "id": draft.ID, "draftVersion": draft.DraftVersion})
	if err = write.Complete(ctx, req.OperationID, applications.Result{Status: 201, Location: "", Data: raw}); err != nil {
		return empty, err
	}
	if err = write.Commit(ctx); err != nil {
		return empty, err
	}
	return draft, nil
}

func (s *Service) Edit(ctx context.Context, principal session.Principal, req EditRequest, metadata applications.Metadata) (MutationResult, error) {
	var result MutationResult
	if s == nil || s.Pool == nil || !validWriteLimits(s.Limits) || !appfields.ValidID(req.AppID) || !appfields.ValidID(req.ViewID) || !appfields.ValidID(req.RecordID) || !appfields.ValidID(req.OperationID) || req.Changes == nil || req.ExpectedSchemaVersion < 0 || req.ExpectedRecordVersion < 1 || metadata.RequestID == "" {
		return result, apprecords.ErrInvalid
	}
	canonical, _ := json.Marshal(struct {
		Kind, AppID, ViewID string
		Body                EditRequest
	}{"record.edit", req.AppID, req.ViewID, req})
	fingerprint := sha256.Sum256(canonical)
	var policy appaccess.Policy
	var fields []appfields.Field
	options := applications.RecordWriteOptions{LockTimeout: s.Limits.LockTimeout, StatementTimeout: s.Limits.StatementTimeout, OperationID: req.OperationID, Kind: "record.edit", Fingerprint: fingerprint,
		Authorize: func(c context.Context, tx pgx.Tx, facts applications.RecordContext) error {
			var menu bool
			policy, menu = policyFor(facts)
			if !menu {
				return applications.ErrDenied
			}
			var err error
			fields, _, err = fieldsInContext(facts)
			if err != nil {
				return err
			}
			// Check the action even for an empty PATCH. The actual target's
			// immutable createdBy is checked later under the row lock by Writer.
			if !policy.CanEdit(facts.Actor.ID, nil) {
				return applications.ErrDenied
			}
			for id := range req.Changes {
				if policy.FieldScope(appaccess.Edit, id) == appaccess.None {
					return applications.ErrDenied
				}
			}
			if !facts.SchemaReady {
				return &appstructure.Error{Code: "APPLICATION_SCHEMA_NOT_READY"}
			}
			return validateNewReferences(c, tx, req.Changes, fields, false)
		},
	}
	write, replayed, err := s.beginRecordMutation(ctx, principal, req.AppID, req.ViewID, req.QueryVersion, options)
	if err != nil {
		return result, err
	}
	defer write.Rollback(context.Background())
	if replayed != nil {
		if err = write.Commit(ctx); err != nil {
			return result, err
		}
		if json.Unmarshal(replayed.Data, &result) != nil {
			return MutationResult{}, ErrUnavailable
		}
		return result, nil
	}
	if err = write.Claim(ctx, req.OperationID, "record.edit", fingerprint); err != nil {
		return result, err
	}
	facts := write.Context()
	if req.ExpectedSchemaVersion != facts.SchemaVersion {
		return result, &appstructure.Error{Code: "APPLICATION_SCHEMA_CONFLICT", Data: map[string]int64{"currentSchemaVersion": facts.SchemaVersion}}
	}
	values, err := normalizeRecordValues(req.Changes, fields, false)
	if err != nil {
		return result, fmt.Errorf("normalize edit: %w", err)
	}
	_, active, err := fieldsInContext(facts)
	if err != nil {
		return result, fmt.Errorf("apply edit: %w", err)
	}
	writer := apprecords.Writer{Gate: appstructure.RecordGate{AppID: req.AppID, TableID: facts.TableID, ViewID: req.ViewID}, Authorization: recordAuthorization{policy: policy}, Fence: appstructure.RecordFence{AppID: req.AppID, TableID: facts.TableID}, Audit: recordAudit{port: appstructure.RecordAudit{Context: facts, OperationID: req.OperationID, BeforeRecordVersion: req.ExpectedRecordVersion, Metadata: metadata}}, DML: controlledDML{}}
	stored, err := writer.EditInTx(ctx, write.Tx(), apprecords.Table{AppID: req.AppID, TableID: facts.TableID, ViewID: req.ViewID, Namespace: "appdata", SchemaVersion: facts.SchemaVersion, Ready: facts.SchemaReady, ActiveFieldIDs: active}, apprecords.Edit{OperationID: req.OperationID, ID: req.RecordID, ActorID: facts.Actor.ID, ExpectedSchemaVersion: req.ExpectedSchemaVersion, ExpectedRecordVersion: req.ExpectedRecordVersion, Changes: values})
	if err != nil {
		return result, fmt.Errorf("apply edit: %w", err)
	}
	if req.DraftRef != nil {
		store := appdrafts.Store{Relation: pgx.Identifier{"applications", "record_drafts"}}
		base := req.ExpectedRecordVersion
		relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(facts.TableID, "-", "")}.Sanitize()
		var targetOwner string
		if err = write.Tx().QueryRow(ctx, "SELECT created_by::text FROM "+relation+" WHERE id=$1", req.RecordID).Scan(&targetOwner); err != nil {
			return MutationResult{}, err
		}
		if err = store.ConsumeInTx(ctx, write.Tx(), draftAccess(facts, policy, fields, &base, targetOwner), req.DraftRef.ID, req.DraftRef.DraftVersion, &req.RecordID, &base); err != nil {
			return MutationResult{}, err
		}
	}
	result = MutationResult{OperationID: stored.OperationID, ID: stored.ID, RecordVersion: stored.RecordVersion, SchemaVersion: stored.SchemaVersion, CreatedAt: stored.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: stored.UpdatedAt.UTC().Format(time.RFC3339Nano)}
	raw, _ := json.Marshal(result)
	if err = write.Complete(ctx, req.OperationID, applications.Result{Status: 200, Location: "", Data: raw}); err != nil {
		return MutationResult{}, err
	}
	if err = write.Commit(ctx); err != nil {
		return MutationResult{}, err
	}
	return result, nil
}
