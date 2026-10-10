package apppresets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appschema"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
	"time"
)

var ErrConflict = errors.New("application preset version conflict")
var ErrNameConflict = errors.New("application preset name conflict")
var ErrLimit = errors.New("application preset limit reached")

type Service struct {
	Pool   *pgxpool.Pool
	Limits appschema.Limits
}
type Request struct {
	AppID, ViewID, ID, OperationID         string
	ExpectedSchemaVersion, ExpectedVersion int64
	State                                  State
}

const maxVersion int64 = 9007199254740991

func validID(id string) bool {
	return appfields.ValidID(id) && id != "00000000-0000-0000-0000-000000000000"
}
func (s *Service) Create(ctx context.Context, p session.Principal, r Request) (applications.Result, error) {
	return s.mutate(ctx, p, r, "preset.create")
}
func (s *Service) Update(ctx context.Context, p session.Principal, r Request) (applications.Result, error) {
	return s.mutate(ctx, p, r, "preset.update")
}
func (s *Service) Delete(ctx context.Context, p session.Principal, r Request) (applications.Result, error) {
	return s.mutate(ctx, p, r, "preset.discard")
}

// Request identity is computed without current schema or grants, so an original
// confirmed operation remains recoverable after legitimate state changes.
func requestHash(r Request, kind string) ([32]byte, error) {
	if !validID(r.AppID) || !validID(r.ViewID) || !validID(r.OperationID) {
		return [32]byte{}, ErrInvalid
	}
	if kind == "preset.create" {
		if r.ID != "" || r.ExpectedVersion != 0 {
			return [32]byte{}, ErrInvalid
		}
	} else if !validID(r.ID) || r.ExpectedVersion < 1 || r.ExpectedVersion > maxVersion {
		return [32]byte{}, ErrInvalid
	}
	identity := map[string]any{"kind": kind, "appId": r.AppID, "viewId": r.ViewID, "id": r.ID, "expectedVersion": r.ExpectedVersion}
	if kind != "preset.discard" {
		if r.ExpectedSchemaVersion < 1 || r.ExpectedSchemaVersion > maxVersion {
			return [32]byte{}, ErrInvalid
		}
		raw, e := json.Marshal(r.State)
		if e != nil {
			return [32]byte{}, ErrInvalid
		}
		state, e := DecodeState(raw)
		if e != nil {
			return [32]byte{}, ErrInvalid
		}
		state.Name = strings.TrimSpace(state.Name)
		raw, e = json.Marshal(state)
		if e != nil {
			return [32]byte{}, ErrInvalid
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		var object any
		if decoder.Decode(&object) != nil {
			return [32]byte{}, ErrInvalid
		}
		identity["state"] = object
		identity["expectedSchemaVersion"] = r.ExpectedSchemaVersion
	}
	raw, e := json.Marshal(identity)
	if e != nil {
		return [32]byte{}, ErrInvalid
	}
	return sha256.Sum256(raw), nil
}
func currentAccess(facts applications.RecordContext) (appaccess.Policy, []appquery.Field, error) {
	p, menu := applications.RecordPolicy(facts)
	if !menu || p.VisibleScope() == appaccess.None {
		return p, nil, applications.ErrDenied
	}
	var definitions []appfields.Field
	if json.Unmarshal(facts.Fields, &definitions) != nil {
		return p, nil, session.ErrUnavailable
	}
	fields := make([]appquery.Field, 0, len(definitions))
	for _, f := range definitions {
		fields = append(fields, appquery.Field{ID: f.ID, Kind: appquery.FieldKind(f.Kind)})
	}
	if _, e := appquery.Compile(nil, nil, fields, 1); e != nil {
		return p, nil, session.ErrUnavailable
	}
	return p, fields, nil
}
func classifyWrite(e error) error {
	var pg *pgconn.PgError
	if errors.As(e, &pg) && pg.Code == "23505" {
		switch pg.ConstraintName {
		case "uq_application_presets_name":
			return ErrNameConflict
		case "uq_application_presets_slot":
			return ErrLimit
		}
	}
	return e
}
func (s *Service) mutate(ctx context.Context, p session.Principal, r Request, kind string) (applications.Result, error) {
	if s == nil || s.Pool == nil || s.Limits.LockTimeout < time.Millisecond || s.Limits.StatementTimeout < time.Millisecond {
		return applications.Result{}, session.ErrUnavailable
	}
	hash, e := requestHash(r, kind)
	if e != nil {
		return applications.Result{}, e
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var prepared Prepared
	options := applications.RecordWriteOptions{LockTimeout: s.Limits.LockTimeout, StatementTimeout: s.Limits.StatementTimeout, OperationID: r.OperationID, Kind: kind, Fingerprint: hash, Authorize: func(_ context.Context, _ pgx.Tx, facts applications.RecordContext) error {
		policy, fields, err := currentAccess(facts)
		if err != nil {
			return err
		}
		if kind == "preset.discard" {
			return nil
		}
		if !facts.SchemaReady {
			return &appstructure.Error{Code: "APPLICATION_SCHEMA_NOT_READY"}
		}
		if facts.SchemaVersion != r.ExpectedSchemaVersion {
			return &appstructure.Error{Code: "APPLICATION_SCHEMA_CONFLICT"}
		}
		prepared, err = Prepare(r.State, policy, fields)
		return err
	}}
	w, e := (&applications.Application{Pool: s.Pool}).BeginRecordWrite(ctx, p, r.AppID, r.ViewID, options)
	if e != nil {
		return applications.Result{}, e
	}
	defer w.Rollback(context.Background())
	old, e := w.Replay(ctx, r.OperationID, kind, hash)
	if e != nil {
		return applications.Result{}, e
	}
	if old != nil {
		if e = w.Commit(ctx); e != nil {
			return applications.Result{}, e
		}
		return *old, nil
	}
	if e = w.Claim(ctx, r.OperationID, kind, hash); e != nil {
		return applications.Result{}, e
	}
	tx := w.Tx()
	id := r.ID
	version := r.ExpectedVersion
	status := 200
	location := ""
	if kind == "preset.create" {
		var slot int
		e = tx.QueryRow(ctx, `SELECT s FROM generate_series(1,20) s WHERE NOT EXISTS(SELECT 1 FROM applications.table_presets p WHERE p.owner_user_id=$1 AND p.app_id=$2 AND p.view_id=$3 AND p.slot=s) ORDER BY s LIMIT 1`, p.UserID, r.AppID, r.ViewID).Scan(&slot)
		if errors.Is(e, pgx.ErrNoRows) {
			return applications.Result{}, ErrLimit
		}
		if e != nil {
			return applications.Result{}, e
		}
		kinds, _ := json.Marshal(prepared.FieldKinds)
		e = tx.QueryRow(ctx, `INSERT INTO applications.table_presets(owner_user_id,app_id,view_id,name,slot,definition_json,field_kinds) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id::text,version`, p.UserID, r.AppID, r.ViewID, prepared.State.Name, slot, string(prepared.Definition), kinds).Scan(&id, &version)
		status = 201
		location = "/api/v1/applications/" + r.AppID + "/forms/" + r.ViewID + "/table-presets/" + id
	} else {
		var actual int64
		e = tx.QueryRow(ctx, `SELECT version FROM applications.table_presets WHERE id=$1 AND owner_user_id=$2 AND app_id=$3 AND view_id=$4 FOR UPDATE`, id, p.UserID, r.AppID, r.ViewID).Scan(&actual)
		if errors.Is(e, pgx.ErrNoRows) {
			return applications.Result{}, applications.ErrMissing
		}
		if e != nil {
			return applications.Result{}, e
		}
		if actual != r.ExpectedVersion {
			return applications.Result{}, ErrConflict
		}
		if kind == "preset.discard" {
			_, e = tx.Exec(ctx, `DELETE FROM applications.table_presets WHERE id=$1 AND owner_user_id=$2 AND app_id=$3 AND view_id=$4`, id, p.UserID, r.AppID, r.ViewID)
			status = 204
		} else {
			if actual >= maxVersion {
				return applications.Result{}, ErrConflict
			}
			version = actual + 1
			kinds, _ := json.Marshal(prepared.FieldKinds)
			_, e = tx.Exec(ctx, `UPDATE applications.table_presets SET name=$5,definition_json=$6,field_kinds=$7,version=$8,updated_at=clock_timestamp() WHERE id=$1 AND owner_user_id=$2 AND app_id=$3 AND view_id=$4`, id, p.UserID, r.AppID, r.ViewID, prepared.State.Name, string(prepared.Definition), kinds, version)
		}
	}
	if e != nil {
		return applications.Result{}, classifyWrite(e)
	}
	data, _ := json.Marshal(map[string]any{"operationId": r.OperationID, "id": id, "version": version})
	result := applications.Result{Status: status, Location: location, Data: data}
	if e = w.Complete(ctx, r.OperationID, result); e != nil {
		return applications.Result{}, e
	}
	if e = w.Commit(ctx); e != nil {
		return applications.Result{}, e
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, p session.Principal, app, view, id string) (json.RawMessage, error) {
	if !validID(id) {
		return nil, ErrInvalid
	}
	items, e := s.read(ctx, p, app, view, id)
	if e != nil {
		return nil, e
	}
	if len(items) != 1 {
		return nil, applications.ErrMissing
	}
	return items[0], nil
}
func (s *Service) List(ctx context.Context, p session.Principal, app, view string) ([]json.RawMessage, error) {
	return s.read(ctx, p, app, view, "")
}
func (s *Service) read(ctx context.Context, p session.Principal, app, view, id string) ([]json.RawMessage, error) {
	if s == nil || s.Pool == nil {
		return nil, session.ErrUnavailable
	}
	if !validID(app) || !validID(view) {
		return nil, ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tx, facts, e := (&applications.Application{Pool: s.Pool}).BeginRecordRead(ctx, p, app, view)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(context.Background())
	policy, fields, e := currentAccess(facts)
	if e != nil {
		return nil, e
	}
	query := `SELECT id::text,name,definition_json,field_kinds,version,created_at,updated_at FROM applications.table_presets WHERE owner_user_id=$1 AND app_id=$2 AND view_id=$3`
	args := []any{p.UserID, app, view}
	if id != "" {
		query += " AND id=$4"
		args = append(args, id)
	}
	query += " ORDER BY updated_at DESC,id ASC LIMIT 21"
	rows, e := tx.Query(ctx, query, args...)
	if e != nil {
		return nil, e
	}
	items := []json.RawMessage{}
	for rows.Next() {
		var itemID, name, definition string
		var signature []byte
		var version int64
		var created, updated time.Time
		if e = rows.Scan(&itemID, &name, &definition, &signature, &version, &created, &updated); e != nil {
			rows.Close()
			return nil, e
		}
		var kinds map[string]appquery.FieldKind
		reason := DefinitionChanged
		if json.Unmarshal(signature, &kinds) == nil {
			reason = CheckStored([]byte(definition), kinds, policy, fields)
		}
		data := map[string]any{"id": itemID, "name": name, "version": version, "invalid": reason != ""}
		if reason != "" {
			data["reason"] = reason
		} else {
			if json.Unmarshal([]byte(definition), &data) != nil {
				rows.Close()
				return nil, session.ErrUnavailable
			}
			data["id"] = itemID
			data["version"] = version
			data["invalid"] = false
			data["appId"] = app
			data["viewId"] = view
			data["createdAt"] = created.UTC().Format(time.RFC3339Nano)
			data["updatedAt"] = updated.UTC().Format(time.RFC3339Nano)
		}
		raw, err := json.Marshal(data)
		if err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, raw)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	if len(items) > 20 {
		return nil, session.ErrUnavailable
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, e
	}
	return items, nil
}
