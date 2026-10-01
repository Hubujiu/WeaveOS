package personnel

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"sort"
	"strings"
	"unicode/utf8"
)

const maxSafeVersion = int64(9007199254740991)

func validID(value string) bool {
	var id pgtype.UUID
	return id.Scan(value) == nil && id.Valid && id.String() == strings.ToLower(value)
}
func uniqueIDs(values []string) []string {
	ids := make([]string, len(values))
	for i, value := range values {
		ids[i] = strings.ToLower(value)
	}
	return unique(ids)
}
func validName(value string) bool {
	return strings.TrimSpace(value) != "" && utf8.RuneCountInString(value) <= 100
}
func unique(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, v := range values {
		if !seen[v] {
			result = append(result, v)
			seen[v] = true
		}
	}
	sort.Strings(result)
	return result
}
func databaseError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && (pg.Code == "40P01" || pg.Code == "40001" || pg.Code == "23503") {
		return ErrConflict
	}
	return err
}
func (a *Application) write(ctx context.Context, p session.Principal) (pgx.Tx, error) {
	if a == nil || a.Pool == nil {
		return nil, session.ErrUnavailable
	}
	tx, err := a.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	if err := a.AuthorizeWrite(ctx, tx, p); err != nil {
		_ = tx.Rollback(context.Background())
		return nil, databaseError(err)
	}
	return tx, nil
}

// Only business writes take revision locks; personal drafts keep their own
// authorization/CAS transaction and never participate in business revisions.
func (a *Application) writeBusiness(ctx context.Context, p session.Principal) (pgx.Tx, error) {
	if a == nil || a.Pool == nil {
		return nil, session.ErrUnavailable
	}
	tx, err := a.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "SELECT personnel.lock_query_revisions()"); err == nil {
		err = a.AuthorizeWrite(ctx, tx, p)
	}
	if err != nil {
		_ = tx.Rollback(context.Background())
		return nil, databaseError(err)
	}
	return tx, nil
}
func (a *Application) read(ctx context.Context, p session.Principal) (pgx.Tx, error) {
	if a == nil || a.Pool == nil {
		return nil, session.ErrUnavailable
	}
	tx, err := a.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	access, err := readAccess(ctx, tx, p)
	if err == nil && !access.PersonnelManage {
		err = ErrDenied
	}
	if err != nil {
		_ = tx.Rollback(context.Background())
		return nil, err
	}
	return tx, nil
}
func appendChange(ctx context.Context, tx pgx.Tx, p session.Principal, meta RequestMetadata, action, kind, id string, before, after any) error {
	summary, err := json.Marshal(struct {
		Before any `json:"before"`
		After  any `json:"after"`
	}{before, after})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO auth.authentication_events(event_type,outcome,actor_user_id,session_ref,reason_code,request_id,client_ip,user_agent,object_type,object_id,change_summary)
 VALUES('personnel_changed','success',$1,NULLIF($2,'')::uuid,$3,$4,NULLIF($5,'')::inet,NULLIF($6,''),$7,$8,$9::jsonb)`, p.UserID, p.SessionRef, action, meta.RequestID, meta.ClientIP, meta.UserAgent, kind, id, summary)
	return err
}
func normalizedPage(query PageQuery) (PageQuery, error) {
	if query.Page == 0 {
		query.Page = 1
	}
	if query.PageSize == 0 {
		query.PageSize = 20
	}
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 || int64(query.Page) > maxSafeVersion/int64(query.PageSize) {
		return query, ErrInvalid
	}
	return query, nil
}
