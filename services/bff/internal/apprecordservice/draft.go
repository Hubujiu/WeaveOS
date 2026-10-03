package apprecordservice

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appdrafts"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
)

func (s *Service) beginDraftMutation(ctx context.Context, principal session.Principal, appID, viewID, operationID, kind string, hash [32]byte) (*applications.RecordWrite, appaccess.Policy, []appfields.Field, *applications.Result, error) {
	var policy appaccess.Policy
	var fields []appfields.Field
	options := applications.RecordWriteOptions{LockTimeout: time.Second, StatementTimeout: 5 * time.Second, OperationID: operationID, Kind: kind, Fingerprint: hash,
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
			if !policy.CanCreate(nil) && policy.VisibleScope() == appaccess.None {
				for _, field := range fields {
					if policy.FieldScope(appaccess.Edit, field.ID) != appaccess.None {
						return nil
					}
				}
				return applications.ErrDenied
			}
			return nil
		},
	}
	write, err := (&applications.Application{Pool: s.Pool}).BeginRecordWrite(ctx, principal, appID, viewID, options)
	if err != nil {
		return nil, policy, nil, nil, err
	}
	replayed, err := write.Replay(ctx, operationID, kind, hash)
	if err != nil {
		write.Rollback(context.Background())
		return nil, policy, nil, nil, err
	}
	if replayed != nil {
		if err = write.Commit(ctx); err != nil {
			return nil, policy, nil, nil, err
		}
		return nil, policy, nil, replayed, nil
	}
	if err = write.Claim(ctx, operationID, kind, hash); err != nil {
		write.Rollback(context.Background())
		return nil, policy, nil, nil, err
	}
	return write, policy, fields, nil, nil
}

func draftStore() appdrafts.Store {
	return appdrafts.Store{Relation: pgx.Identifier{"applications", "record_drafts"}}
}

func draftBinding(ctx context.Context, tx pgx.Tx, facts applications.RecordContext, id string) (*string, *int64, int64, error) {
	var target *string
	var base *int64
	var schema int64
	err := tx.QueryRow(ctx, `SELECT target_record_id::text,base_record_version,schema_version FROM applications.record_drafts
 WHERE id=$1 AND owner_user_id=$2 AND app_id=$3 AND table_id=$4 AND view_id=$5`, id, facts.Actor.ID, facts.App.ID, facts.TableID, facts.ViewID).Scan(&target, &base, &schema)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, 0, appdrafts.ErrMissing
	}
	return target, base, schema, err
}

func currentDraftBase(ctx context.Context, tx pgx.Tx, tableID string, target *string) (*int64, string, error) {
	if target == nil {
		return nil, "", nil
	}
	if !appfields.ValidID(*target) || !appfields.ValidID(tableID) {
		return nil, "", appdrafts.ErrInvalid
	}
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(tableID, "-", "")}.Sanitize()
	var version int64
	var owner string
	err := tx.QueryRow(ctx, "SELECT record_version,created_by::text FROM "+relation+" WHERE id=$1", *target).Scan(&version, &owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", appdrafts.ErrBaseConflict
	}
	if err != nil {
		return nil, "", err
	}
	return &version, owner, nil
}

func (s *Service) UpdateDraft(ctx context.Context, principal session.Principal, req DraftUpdateRequest, metadata applications.Metadata) (appdrafts.Draft, error) {
	var empty appdrafts.Draft
	if s == nil || s.Pool == nil || !appfields.ValidID(req.AppID) || !appfields.ValidID(req.ViewID) || !appfields.ValidID(req.DraftID) || !appfields.ValidID(req.OperationID) || req.ExpectedDraftVersion < 1 || metadata.RequestID == "" {
		return empty, appdrafts.ErrInvalid
	}
	canonical, _ := json.Marshal(struct {
		Kind string
		Body DraftUpdateRequest
	}{"draft.update", req})
	hash := sha256.Sum256(canonical)
	write, policy, fields, replayed, err := s.beginDraftMutation(ctx, principal, req.AppID, req.ViewID, req.OperationID, "draft.update", hash)
	if err != nil {
		return empty, err
	}
	if replayed != nil {
		var m struct {
			ID      string `json:"id"`
			Version int64  `json:"draftVersion"`
		}
		if json.Unmarshal(replayed.Data, &m) != nil {
			return empty, ErrUnavailable
		}
		return appdrafts.Draft{ID: m.ID, DraftVersion: m.Version}, nil
	}
	defer write.Rollback(context.Background())
	facts := write.Context()
	if err = (appstructure.RecordGate{AppID: req.AppID, TableID: facts.TableID, ViewID: req.ViewID}).LockTable(ctx, write.Tx(), facts.TableID, facts.SchemaVersion); err != nil {
		return empty, err
	}
	target, base, schema, err := draftBinding(ctx, write.Tx(), facts, req.DraftID)
	if err != nil {
		return empty, err
	}
	current, owner, err := currentDraftBase(ctx, write.Tx(), facts.TableID, target)
	if err != nil {
		return empty, err
	}
	if schema != facts.SchemaVersion || target != nil && (base == nil || current == nil || *base != *current) {
		return empty, appdrafts.ErrBaseConflict
	}
	access := draftAccess(facts, policy, fields, current, owner)
	draft, err := draftStore().UpdateInTx(ctx, write.Tx(), access, req.DraftID, appdrafts.Update{ExpectedDraftVersion: req.ExpectedDraftVersion, Changes: req.Changes, RemoveFieldIDs: req.RemoveFieldIDs})
	if err != nil {
		return empty, err
	}
	raw, _ := json.Marshal(map[string]any{"operationId": req.OperationID, "id": draft.ID, "draftVersion": draft.DraftVersion})
	if err = write.Complete(ctx, req.OperationID, applications.Result{Status: 200, Location: "", Data: raw}); err != nil {
		return empty, err
	}
	if err = write.Commit(ctx); err != nil {
		return empty, err
	}
	return draft, nil
}

func (s *Service) DiscardDraft(ctx context.Context, principal session.Principal, req DraftDiscardRequest, metadata applications.Metadata) error {
	if s == nil || s.Pool == nil || !appfields.ValidID(req.AppID) || !appfields.ValidID(req.ViewID) || !appfields.ValidID(req.DraftID) || !appfields.ValidID(req.OperationID) || req.ExpectedDraftVersion < 1 || metadata.RequestID == "" {
		return appdrafts.ErrInvalid
	}
	canonical, _ := json.Marshal(struct {
		Kind string
		Body DraftDiscardRequest
	}{"draft.discard", req})
	hash := sha256.Sum256(canonical)
	write, policy, fields, replayed, err := s.beginDraftMutation(ctx, principal, req.AppID, req.ViewID, req.OperationID, "draft.discard", hash)
	if err != nil {
		return err
	}
	if replayed != nil {
		return nil
	}
	defer write.Rollback(context.Background())
	facts := write.Context()
	if err = (appstructure.RecordGate{AppID: req.AppID, TableID: facts.TableID, ViewID: req.ViewID}).LockTable(ctx, write.Tx(), facts.TableID, facts.SchemaVersion); err != nil {
		return err
	}
	target, _, _, err := draftBinding(ctx, write.Tx(), facts, req.DraftID)
	if err != nil {
		return err
	}
	current, owner, err := currentDraftBase(ctx, write.Tx(), facts.TableID, target)
	if err != nil {
		return err
	}
	if err = draftStore().DiscardInTx(ctx, write.Tx(), draftAccess(facts, policy, fields, current, owner), req.DraftID, req.ExpectedDraftVersion); err != nil {
		return err
	}
	raw, _ := json.Marshal(map[string]any{"operationId": req.OperationID, "id": req.DraftID, "draftVersion": req.ExpectedDraftVersion})
	if err = write.Complete(ctx, req.OperationID, applications.Result{Status: 204, Location: "", Data: raw}); err != nil {
		return err
	}
	return write.Commit(ctx)
}
