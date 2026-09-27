package audit

import (
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
)

type Reader struct {
	Authentication *session.Authenticator
	Live           *pgxpool.Pool
}

func (r *Reader) List(request *http.Request, limit int) ([]json.RawMessage, error) {
	if r == nil || r.Authentication == nil || r.Live == nil || request == nil {
		return nil, session.ErrUnavailable
	}
	if limit < 1 || limit > 1000 {
		return nil, errors.New("audit limit must be 1 through 1000")
	}
	p, err := r.Authentication.Authenticate(request, false)
	if err != nil {
		return nil, err
	}
	if !p.BootstrapAdmin {
		return nil, session.ErrForbidden
	}
	tx, err := r.Live.BeginTx(request.Context(), pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, session.ErrUnavailable
	}
	defer tx.Rollback(request.Context())
	// Fresh current-state checkpoint shares the audit read snapshot.
	var allowed bool
	if err := tx.QueryRow(request.Context(), "SELECT status='active' AND is_bootstrap_admin AND auth_version::text=$2 FROM auth.users WHERE id=$1", p.UserID, p.Record.AuthVersion).Scan(&allowed); err != nil || !allowed {
		return nil, session.ErrForbidden
	}
	rows, err := tx.Query(request.Context(), "SELECT to_jsonb(e) FROM auth.authentication_events e WHERE occurred_at > current_timestamp - interval '1 year' ORDER BY occurred_at DESC,id DESC LIMIT $1", limit)
	if err != nil {
		return nil, session.ErrUnavailable
	}
	events := []json.RawMessage{}
	for rows.Next() {
		var e []byte
		if err := rows.Scan(&e); err != nil {
			rows.Close()
			return nil, session.ErrUnavailable
		}
		events = append(events, json.RawMessage(e))
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, session.ErrUnavailable
	}
	if err := tx.Commit(request.Context()); err != nil {
		return nil, session.ErrUnavailable
	}
	return events, nil
}
