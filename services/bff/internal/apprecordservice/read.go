package apprecordservice

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appdrafts"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apppolicy"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
)

func (s *Service) GetRecord(ctx context.Context, principal session.Principal, appID, viewID, id string) (Record, error) {
	var empty Record
	if s == nil || s.Pool == nil || !appfields.ValidID(appID) || !appfields.ValidID(viewID) || !appfields.ValidID(id) {
		return empty, applications.ErrResourceInvalid
	}
	strategy := &recordStrategy{service: s, principal: principal, appID: appID, viewID: viewID}
	tx, err := strategy.OpenRead(ctx)
	if err != nil {
		return empty, err
	}
	defer tx.Rollback(context.Background())
	raw, _ := json.Marshal(criteria{Filter: json.RawMessage("null"), Sort: json.RawMessage("null")})
	base, _, args, err := strategy.selectSQL(raw, false)
	if err != nil {
		return empty, err
	}
	query := base + " AND r.id=$" + strconv.Itoa(len(args)+1) + "::uuid"
	args = append(args, id)
	var encoded []byte
	err = tx.QueryRow(ctx, query, args...).Scan(&encoded)
	if errors.Is(err, pgx.ErrNoRows) {
		return empty, applications.ErrMissing
	}
	if err != nil {
		return empty, err
	}
	var item Record
	if err = json.Unmarshal(encoded, &item); err != nil {
		return empty, err
	}
	item.AppID, item.TableID, item.ViewID, item.SchemaVersion = strategy.appID, strategy.tableID, strategy.viewID, strategy.control.Schema
	items := []Record{item}
	if err = strategy.hydrateReferences(ctx, tx, items); err != nil {
		return empty, err
	}
	if err = tx.Commit(ctx); err != nil {
		return empty, err
	}
	return items[0], nil
}

func (s *Service) draftRead(ctx context.Context, p session.Principal, appID, viewID string) (*recordStrategy, pgx.Tx, applications.RecordContext, error) {
	if s == nil || s.Pool == nil || !appfields.ValidID(appID) || !appfields.ValidID(viewID) {
		return nil, nil, applications.RecordContext{}, applications.ErrResourceInvalid
	}
	strategy := &recordStrategy{service: s, principal: p, appID: appID, viewID: viewID, draft: true}
	tx, err := strategy.OpenRead(ctx)
	if err != nil {
		return nil, nil, applications.RecordContext{}, err
	}
	facts := applications.RecordContext{Actor: apppolicy.TrustedActor{ID: p.UserID, BootstrapAdmin: strategy.policy.BootstrapAdmin},
		App: applications.App{ID: appID, OwnerUserID: strategy.ownerID, PolicyRevision: strategy.control.Policy}, ViewID: viewID, TableID: strategy.tableID, SchemaVersion: strategy.control.Schema, SchemaReady: true}
	return strategy, tx, facts, nil
}

func (s *Service) GetDraft(ctx context.Context, p session.Principal, appID, viewID, id string) (appdrafts.Draft, error) {
	var empty appdrafts.Draft
	if !appfields.ValidID(id) {
		return empty, appdrafts.ErrInvalid
	}
	strategy, tx, facts, err := s.draftRead(ctx, p, appID, viewID)
	if err != nil {
		return empty, err
	}
	defer tx.Rollback(context.Background())
	target, _, _, err := draftBinding(ctx, tx, facts, id)
	if err != nil {
		return empty, err
	}
	base, owner, err := currentDraftBase(ctx, tx, facts.TableID, target)
	if err != nil {
		return empty, err
	}
	access := draftAccess(facts, strategy.policy, strategy.definitions, base, owner)
	draft, err := draftStore().GetInTx(ctx, tx, access, id)
	if err != nil {
		return empty, err
	}
	if err = tx.Commit(ctx); err != nil {
		return empty, err
	}
	return draft, nil
}

type draftBaseLookup struct{ tableID string }

func (b draftBaseLookup) CurrentBases(ctx context.Context, tx pgx.Tx, ids []string) (map[string]appdrafts.Base, error) {
	result := make(map[string]appdrafts.Base, len(ids))
	for _, id := range ids {
		if !appfields.ValidID(id) {
			return nil, appdrafts.ErrInvalid
		}
		result[id] = appdrafts.Base{}
	}
	if len(ids) == 0 {
		return result, nil
	}
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(b.tableID, "-", "")}.Sanitize()
	rows, err := tx.Query(ctx, "SELECT id::text,record_version,created_by::text FROM "+relation+" WHERE id=ANY($1::uuid[])", ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var version int64
		var owner string
		if err = rows.Scan(&id, &version, &owner); err != nil {
			return nil, err
		}
		v := version
		result[id] = appdrafts.Base{Version: &v, OwnerID: owner}
	}
	return result, rows.Err()
}
func (s *Service) ListDrafts(ctx context.Context, p session.Principal, req DraftListRequest) (appdrafts.Page, error) {
	var empty appdrafts.Page
	strategy, tx, facts, err := s.draftRead(ctx, p, req.AppID, req.ViewID)
	if err != nil {
		return empty, err
	}
	defer tx.Rollback(context.Background())
	access := draftAccess(facts, strategy.policy, strategy.definitions, nil, "")
	access.ForTarget = func(target *string, owner string) appdrafts.Access {
		if target == nil {
			return draftAccess(facts, strategy.policy, strategy.definitions, nil, "")
		}
		return draftAccess(facts, strategy.policy, strategy.definitions, nil, owner)
	}
	if !access.ResourceAllowed {
		for _, field := range strategy.definitions {
			if strategy.policy.FieldScope(appaccess.Edit, field.ID) != appaccess.None {
				access.ResourceAllowed = true
				break
			}
		}
	}
	page, err := draftStore().ListInTx(ctx, tx, access, draftBaseLookup{facts.TableID}, req.PageSize, req.PageToken)
	if err != nil {
		return empty, err
	}
	if err = tx.Commit(ctx); err != nil {
		return empty, err
	}
	return page, nil
}
