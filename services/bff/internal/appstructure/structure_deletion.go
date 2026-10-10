package appstructure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5/pgconn"
	"io"
	"mime"
	"net/http"
	"strconv"
	"unicode/utf8"
)

type StructureDeletionInput struct {
	OperationID              string `json:"operationId"`
	ExpectedStructureVersion int64  `json:"expectedStructureVersion"`
	ExpectedResourceVersion  int64  `json:"expectedResourceVersion"`
}

func (s *Service) structureDeletion(w http.ResponseWriter, r *http.Request, p session.Principal, app, kind, id string) {
	media, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || media != "application/json" {
		respond(w, r, 415, "COMMON_UNSUPPORTED_MEDIA_TYPE", nil)
		return
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 4096))
	if e != nil || !utf8.Valid(raw) {
		fail(w, r, invalid(), "")
		return
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	if jsonValue(d) != nil {
		fail(w, r, invalid(), "")
		return
	}
	if _, e = d.Token(); e != io.EOF {
		fail(w, r, invalid(), "")
		return
	}
	if _, e = object(raw, []string{"operationId", "expectedStructureVersion", "expectedResourceVersion"}, nil, nil); e != nil {
		fail(w, r, e, "")
		return
	}
	var in StructureDeletionInput
	if json.Unmarshal(raw, &in) != nil {
		fail(w, r, invalid(), "")
		return
	}
	out, e := s.Application.DeleteStructure(r.Context(), p, app, kind, id, in)
	if e != nil {
		fail(w, r, e, in.OperationID)
		return
	}
	s.finish(w, r, p, out, true)
}
func (a *Application) DeleteStructure(c context.Context, p session.Principal, app, kind, id string, in StructureDeletionInput) (applications.Result, error) {
	var out applications.Result
	for _, v := range []string{app, id, in.OperationID} {
		if !appfields.ValidID(v) || v == "00000000-0000-0000-0000-000000000000" {
			return out, invalid()
		}
	}
	if in.ExpectedStructureVersion < 0 || in.ExpectedStructureVersion > maxVersion || in.ExpectedResourceVersion < 0 || in.ExpectedResourceVersion > maxVersion {
		return out, invalid()
	}
	switch kind {
	case "application":
		if id != app || in.ExpectedResourceVersion == 0 {
			return out, invalid()
		}
	case "directory":
		if in.ExpectedResourceVersion != 0 {
			return out, invalid()
		}
	case "table", "form":
	default:
		return out, invalid()
	}
	if a == nil || a.Pool == nil {
		return out, ErrUnavailable
	}
	raw, _ := json.Marshal(struct {
		App, Kind, ID string
		Input         StructureDeletionInput
	}{app, kind, id, in})
	hash := sha256.Sum256(raw)
	operationKind := kind + ".delete"
	mw, e := (&applications.Application{Pool: a.Pool}).BeginStructureDeletion(c, p, app, applications.StructureDeletionOptions{WriteOptions: applications.WriteOptions{LockTimeout: a.Limits.LockTimeout, StatementTimeout: a.Limits.StatementTimeout}, OperationID: in.OperationID, Kind: operationKind, Fingerprint: hash})
	if e != nil {
		return out, e
	}
	defer mw.Rollback(context.Background())
	old, e := mw.Replay(c, in.OperationID, operationKind, hash)
	if e != nil {
		return out, e
	}
	if old != nil {
		return *old, mw.Commit(c)
	}
	if e = mw.Claim(c, in.OperationID, operationKind, hash); e != nil {
		return out, e
	}
	e = mw.Tx().QueryRow(c, "SELECT applications.delete_structure_resource($1,$2,$3,$4,$5,$6,$7)", app, kind, id, p.UserID, in.OperationID, in.ExpectedStructureVersion, in.ExpectedResourceVersion).Scan(&out.Data)
	if e != nil {
		return applications.Result{}, structureDeletionError(e)
	}
	out.Status = 200
	if e = mw.Complete(c, in.OperationID, out); e != nil {
		return applications.Result{}, e
	}
	if e = mw.Commit(c); e != nil {
		return applications.Result{}, e
	}
	return out, nil
}
func structureDeletionError(e error) error {
	var p *pgconn.PgError
	if errors.As(e, &p) {
		switch p.Code {
		case "P0002":
			return applications.ErrMissing
		case "42501":
			return applications.ErrDenied
		case "W0037":
			var deps []string
			if json.Unmarshal([]byte(p.Detail), &deps) != nil || len(deps) > 8 {
				return ErrUnavailable
			}
			return &Error{Code: "APPLICATION_STRUCTURE_NOT_EMPTY", Data: map[string]any{"dependencies": deps}}
		case "W0038":
			version, err := strconv.ParseInt(p.Detail, 10, 64)
			if err != nil || version < 0 || version > maxVersion {
				return ErrUnavailable
			}
			return &Error{Code: "APPLICATION_STRUCTURE_CONFLICT", Data: map[string]int64{"currentStructureVersion": version}}
		case "W0039":
			version, err := strconv.ParseInt(p.Detail, 10, 64)
			if err != nil || version < 0 || version > maxVersion {
				return ErrUnavailable
			}
			return &Error{Code: "APPLICATION_VIEW_CONFLICT", Data: map[string]int64{"currentViewVersion": version}}
		case "W0002":
			version, err := strconv.ParseInt(p.Detail, 10, 64)
			if err != nil || version < 0 || version > maxVersion {
				return ErrUnavailable
			}
			return &Error{Code: "APPLICATION_SCHEMA_CONFLICT", Data: map[string]int64{"currentSchemaVersion": version}}
		case "W0040":
			return &Error{Code: "APPLICATION_POLICY_CONFLICT"}
		case "23514", "22P02":
			return invalid()
		}
	}
	return ErrUnavailable
}
