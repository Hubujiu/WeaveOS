// Package appdrafts stores incomplete form input independently of typed records.
// Every write accepts a caller-owned transaction; the shared operation ledger,
// live Session and policy snapshot must be established before calling it.
package appdrafts

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

var ErrInvalid = errors.New("invalid draft input")
var ErrForbidden = errors.New("draft resource forbidden")
var ErrMissing = errors.New("draft not found")
var ErrConflict = errors.New("draft version conflict")
var ErrBaseConflict = errors.New("draft base conflict")
var ErrUnavailable = errors.New("draft dependency unavailable")

type Values map[string]any
type FieldStatus struct {
	Exists, Writable bool
	Kind             string // live V030-013 kind; complete business validation is deferred
	MaxRunes         int    // current field-specific string limit
	MaxItems         int    // current multi_select item limit
}
type Access struct {
	ActorID, AppID, TableID, ViewID string
	ResourceAllowed                 bool
	CurrentSchemaVersion            int64
	CurrentBaseRecordVersion        *int64
	Field                           func(string) FieldStatus
	ForTarget                       func(*string, string) Access
}
type Draft struct {
	ID                string     `json:"id"`
	ViewID            string     `json:"viewId"`
	TableID           string     `json:"tableId"`
	TargetRecordID    *string    `json:"targetRecordId"`
	SchemaVersion     int64      `json:"schemaVersion"`
	BaseRecordVersion *int64     `json:"baseRecordVersion"`
	DraftVersion      int64      `json:"draftVersion"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
	HasConflicts      bool       `json:"hasConflicts"`
	Values            Values     `json:"values"`
	Conflicts         []Conflict `json:"conflicts"`
}
type Conflict struct {
	FieldID *string `json:"fieldId"`
	Reason  string  `json:"reason"`
}
type Create struct {
	ID                string
	TargetRecordID    *string
	SchemaVersion     int64
	BaseRecordVersion *int64
	Values            Values
}
type Update struct {
	ExpectedDraftVersion int64
	Changes              Values
	RemoveFieldIDs       []string
}
type Store struct{ Relation pgx.Identifier }
type Summary struct {
	ID                string    `json:"id"`
	ViewID            string    `json:"viewId"`
	TableID           string    `json:"tableId"`
	TargetRecordID    *string   `json:"targetRecordId"`
	SchemaVersion     int64     `json:"schemaVersion"`
	BaseRecordVersion *int64    `json:"baseRecordVersion"`
	DraftVersion      int64     `json:"draftVersion"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
	HasConflicts      bool      `json:"hasConflicts"`
}
type Page struct {
	Items     []Summary
	NextToken string
}
type BaseLookup interface {
	CurrentBases(context.Context, pgx.Tx, []string) (map[string]Base, error)
}
type Base struct {
	Version *int64
	OwnerID string
}
type cursor struct {
	UpdatedAt time.Time `json:"updatedAt"`
	ID        string    `json:"id"`
}

func parseCursor(token string) (*cursor, error) {
	if token == "" {
		return nil, nil
	}
	if len(token) > 512 {
		return nil, ErrInvalid
	}
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || base64.RawURLEncoding.EncodeToString(b) != token {
		return nil, ErrInvalid
	}
	var c cursor
	if json.Unmarshal(b, &c) != nil || !uuid.MatchString(c.ID) || c.UpdatedAt.IsZero() {
		return nil, ErrInvalid
	}
	return &c, nil
}
func (s Store) ListInTx(ctx context.Context, tx pgx.Tx, access Access, bases BaseLookup, pageSize int, pageToken string) (Page, error) {
	result := Page{Items: []Summary{}}
	if err := validAccess(access); err != nil {
		return result, err
	}
	if pageSize < 1 || pageSize > 100 {
		return result, ErrInvalid
	}
	c, err := parseCursor(pageToken)
	if err != nil {
		return result, err
	}
	rel, err := s.relation()
	if err != nil {
		return result, err
	}
	var since any
	var last any
	if c != nil {
		since = c.UpdatedAt
		last = c.ID
	}
	rows, err := tx.Query(ctx, `SELECT `+draftColumns+` FROM `+rel+` WHERE owner_user_id=$1 AND app_id=$2 AND table_id=$3 AND view_id=$4
 AND ($5::timestamptz IS NULL OR (updated_at,id)<($5::timestamptz,$6::uuid)) ORDER BY updated_at DESC,id DESC LIMIT $7`, access.ActorID, access.AppID, access.TableID, access.ViewID, since, last, pageSize+1)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	page := make([]stored, 0, pageSize+1)
	for rows.Next() {
		d, e := scan(rows)
		if e != nil {
			return result, e
		}
		page = append(page, d)
	}
	if err = rows.Err(); err != nil {
		return result, err
	}
	if len(page) > pageSize {
		page = page[:pageSize]
		last := page[len(page)-1]
		b, e := json.Marshal(cursor{UpdatedAt: last.UpdatedAt.UTC(), ID: last.ID})
		if e != nil {
			return result, e
		}
		result.NextToken = base64.RawURLEncoding.EncodeToString(b)
	}
	targets := map[string]bool{}
	for _, d := range page {
		if d.TargetRecordID != nil {
			targets[*d.TargetRecordID] = true
		}
	}
	currentBases := map[string]Base{}
	if len(targets) > 0 {
		if bases == nil {
			return Page{}, ErrUnavailable
		}
		ids := make([]string, 0, len(targets))
		for id := range targets {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		currentBases, err = bases.CurrentBases(ctx, tx, ids)
		if err != nil {
			return Page{}, err
		}
		if currentBases == nil {
			return Page{}, ErrUnavailable
		}
	}
	for _, d := range page {
		current := access
		base := Base{}
		if d.TargetRecordID != nil {
			base = currentBases[*d.TargetRecordID]
		}
		if access.ForTarget != nil {
			current = access.ForTarget(d.TargetRecordID, base.OwnerID)
		}
		if d.TargetRecordID != nil {
			current.CurrentBaseRecordVersion = base.Version
		} else {
			current.CurrentBaseRecordVersion = nil
		}
		v := visible(d, current)
		result.Items = append(result.Items, Summary{ID: v.ID, ViewID: v.ViewID, TableID: v.TableID, TargetRecordID: v.TargetRecordID, SchemaVersion: v.SchemaVersion, BaseRecordVersion: v.BaseRecordVersion, DraftVersion: v.DraftVersion, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, HasConflicts: v.HasConflicts})
	}
	return result, nil
}

var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

const maxVersion int64 = 9007199254740991
const maxDraftBytes = 1 << 20

func (s Store) relation() (string, error) {
	if len(s.Relation) != 2 || s.Relation[0] == "" || s.Relation[1] == "" {
		return "", ErrInvalid
	}
	return s.Relation.Sanitize(), nil
}
func validAccess(a Access) error {
	if !uuid.MatchString(a.ActorID) || !uuid.MatchString(a.AppID) || !uuid.MatchString(a.TableID) || !uuid.MatchString(a.ViewID) || a.Field == nil || a.CurrentSchemaVersion < 1 || a.CurrentSchemaVersion > maxVersion {
		return ErrInvalid
	}
	if !a.ResourceAllowed {
		return ErrForbidden
	}
	return nil
}
func validateValues(v Values, a Access) error {
	if v == nil {
		return ErrInvalid
	}
	for id, value := range v {
		field := a.Field(id)
		if !uuid.MatchString(id) || !field.Exists || !validKind(field.Kind) {
			return ErrInvalid
		}
		if !field.Writable {
			return ErrForbidden
		}
		switch x := value.(type) {
		case nil:
		case bool:
			if field.Kind != "boolean" {
				return ErrInvalid
			}
		case string:
			if field.Kind == "boolean" || field.Kind == "multi_select" || field.Kind == "" || field.MaxRunes < 1 {
				return ErrInvalid
			}
			if !utf8.ValidString(x) || strings.ContainsRune(x, 0) || utf8.RuneCountInString(x) > field.MaxRunes {
				return ErrInvalid
			}
		case []string:
			if field.Kind != "multi_select" || field.MaxItems < 1 || field.MaxRunes < 1 || len(x) > field.MaxItems {
				return ErrInvalid
			}
			for _, s := range x {
				if !utf8.ValidString(s) || strings.ContainsRune(s, 0) || utf8.RuneCountInString(s) > field.MaxRunes {
					return ErrInvalid
				}
			}
		default:
			return ErrInvalid
		}
	}
	b, err := json.Marshal(v)
	if err != nil || len(b) > maxDraftBytes {
		return ErrInvalid
	}
	return nil
}
func validKind(kind string) bool {
	switch kind {
	case "text", "multiline", "number", "money", "date", "datetime", "boolean", "single_select", "multi_select", "member", "department":
		return true
	}
	return false
}
func version(v int64) bool           { return v >= 1 && v <= maxVersion }
func sameNullable(a, b *string) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }
func sameVersion(a, b *int64) bool   { return a == nil && b == nil || a != nil && b != nil && *a == *b }
func validBinding(target *string, base *int64) bool {
	if (target == nil) != (base == nil) {
		return false
	}
	if target != nil {
		return uuid.MatchString(*target) && version(*base)
	}
	return true
}

type stored struct {
	Draft
	payload Values
}

const draftColumns = `id::text,view_id::text,table_id::text,target_record_id::text,schema_version,base_record_version,draft_version,created_at,updated_at,values_json`

func scan(row pgx.Row) (stored, error) {
	var d stored
	var raw []byte
	err := row.Scan(&d.ID, &d.ViewID, &d.TableID, &d.TargetRecordID, &d.SchemaVersion, &d.BaseRecordVersion, &d.DraftVersion, &d.CreatedAt, &d.UpdatedAt, &raw)
	if err != nil {
		return d, err
	}
	var encoded map[string]json.RawMessage
	if err = json.Unmarshal(raw, &encoded); err != nil {
		return d, err
	}
	if encoded == nil {
		return d, ErrInvalid
	}
	d.payload = Values{}
	for id, b := range encoded {
		switch {
		case string(b) == "null":
			d.payload[id] = nil
		case len(b) > 0 && b[0] == '"':
			var v string
			if err = json.Unmarshal(b, &v); err != nil {
				return d, err
			}
			d.payload[id] = v
		case string(b) == "true" || string(b) == "false":
			var v bool
			if err = json.Unmarshal(b, &v); err != nil {
				return d, err
			}
			d.payload[id] = v
		case len(b) > 0 && b[0] == '[':
			var v []string
			if err = json.Unmarshal(b, &v); err != nil {
				return d, err
			}
			d.payload[id] = v
		default:
			return d, ErrInvalid
		}
	}
	return d, nil
}
func (s Store) owned(ctx context.Context, tx pgx.Tx, a Access, id string, lock bool) (stored, error) {
	rel, err := s.relation()
	if err != nil {
		return stored{}, err
	}
	if !uuid.MatchString(id) {
		return stored{}, ErrInvalid
	}
	query := `SELECT ` + draftColumns + ` FROM ` + rel + ` WHERE id=$1 AND owner_user_id=$2 AND app_id=$3 AND table_id=$4 AND view_id=$5`
	if lock {
		query += ` FOR UPDATE`
	}
	d, err := scan(tx.QueryRow(ctx, query, id, a.ActorID, a.AppID, a.TableID, a.ViewID))
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrMissing
	}
	return d, err
}
func visible(d stored, a Access) Draft {
	out := d.Draft
	out.Values = Values{}
	out.Conflicts = []Conflict{}
	if d.SchemaVersion != a.CurrentSchemaVersion {
		out.Conflicts = append(out.Conflicts, Conflict{Reason: "SCHEMA_CHANGED"})
	}
	if d.TargetRecordID != nil && !sameVersion(d.BaseRecordVersion, a.CurrentBaseRecordVersion) {
		out.Conflicts = append(out.Conflicts, Conflict{Reason: "BASE_RECORD_CHANGED"})
	}
	keys := make([]string, 0, len(d.payload))
	for id := range d.payload {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	for _, id := range keys {
		v := d.payload[id]
		field := a.Field(id)
		if !field.Exists {
			key := id
			out.Conflicts = append(out.Conflicts, Conflict{FieldID: &key, Reason: "FIELD_REMOVED"})
			continue
		}
		if !field.Writable {
			key := id
			out.Conflicts = append(out.Conflicts, Conflict{FieldID: &key, Reason: "FIELD_PERMISSION_REVOKED"})
			continue
		}
		out.Values[id] = v
	}
	out.HasConflicts = len(out.Conflicts) > 0
	return out
}
func (s Store) CreateInTx(ctx context.Context, tx pgx.Tx, access Access, input Create) (Draft, error) {
	if err := validAccess(access); err != nil {
		return Draft{}, err
	}
	if !uuid.MatchString(input.ID) || !version(input.SchemaVersion) || !validBinding(input.TargetRecordID, input.BaseRecordVersion) {
		return Draft{}, ErrInvalid
	}
	if input.SchemaVersion != access.CurrentSchemaVersion || !sameVersion(input.BaseRecordVersion, access.CurrentBaseRecordVersion) {
		return Draft{}, ErrBaseConflict
	}
	if err := validateValues(input.Values, access); err != nil {
		return Draft{}, err
	}
	rel, err := s.relation()
	if err != nil {
		return Draft{}, err
	}
	raw, err := json.Marshal(input.Values)
	if err != nil {
		return Draft{}, err
	}
	d, err := scan(tx.QueryRow(ctx, `INSERT INTO `+rel+`(id,owner_user_id,app_id,table_id,view_id,target_record_id,schema_version,base_record_version,draft_version,values_json)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,1,$9) RETURNING `+draftColumns, input.ID, access.ActorID, access.AppID, access.TableID, access.ViewID, input.TargetRecordID, input.SchemaVersion, input.BaseRecordVersion, raw))
	if err != nil {
		return Draft{}, err
	}
	return visible(d, access), nil
}
func (s Store) GetInTx(ctx context.Context, tx pgx.Tx, access Access, id string) (Draft, error) {
	if err := validAccess(access); err != nil {
		return Draft{}, err
	}
	d, err := s.owned(ctx, tx, access, id, false)
	if err != nil {
		return Draft{}, err
	}
	return visible(d, access), nil
}
func (s Store) UpdateInTx(ctx context.Context, tx pgx.Tx, access Access, id string, input Update) (Draft, error) {
	if err := validAccess(access); err != nil {
		return Draft{}, err
	}
	if !version(input.ExpectedDraftVersion) {
		return Draft{}, ErrInvalid
	}
	d, err := s.owned(ctx, tx, access, id, true)
	if err != nil {
		return Draft{}, err
	}
	if d.DraftVersion != input.ExpectedDraftVersion {
		return Draft{}, ErrConflict
	}
	changes := input.Changes
	if changes == nil {
		changes = Values{}
	}
	if err := validateValues(changes, access); err != nil {
		return Draft{}, err
	}
	removed := map[string]bool{}
	for _, field := range input.RemoveFieldIDs {
		if !uuid.MatchString(field) || removed[field] {
			return Draft{}, ErrInvalid
		}
		if _, overlap := changes[field]; overlap {
			return Draft{}, ErrInvalid
		}
		removed[field] = true
	}
	if len(changes) == 0 && len(removed) == 0 {
		return visible(d, access), nil
	}
	for id, v := range changes {
		d.payload[id] = v
	}
	for id := range removed {
		delete(d.payload, id)
	}
	raw, err := json.Marshal(d.payload)
	if err != nil || len(raw) > maxDraftBytes {
		return Draft{}, ErrInvalid
	}
	rel, err := s.relation()
	if err != nil {
		return Draft{}, err
	}
	d, err = scan(tx.QueryRow(ctx, `UPDATE `+rel+` SET values_json=$1,draft_version=draft_version+1,updated_at=now() WHERE id=$2 AND owner_user_id=$3 AND draft_version=$4 RETURNING `+draftColumns, raw, id, access.ActorID, input.ExpectedDraftVersion))
	if err != nil {
		return Draft{}, err
	}
	return visible(d, access), nil
}
func (s Store) ConsumeInTx(ctx context.Context, tx pgx.Tx, access Access, id string, draftVersion int64, targetID *string, baseVersion *int64) error {
	if err := validAccess(access); err != nil {
		return err
	}
	if !uuid.MatchString(id) || !version(draftVersion) || !validBinding(targetID, baseVersion) {
		return ErrInvalid
	}
	if !sameVersion(baseVersion, access.CurrentBaseRecordVersion) {
		return ErrBaseConflict
	}
	d, err := s.owned(ctx, tx, access, id, true)
	if err != nil {
		return err
	}
	if d.SchemaVersion != access.CurrentSchemaVersion || !sameVersion(d.BaseRecordVersion, baseVersion) || !sameNullable(d.TargetRecordID, targetID) {
		return ErrBaseConflict
	}
	if d.DraftVersion != draftVersion {
		return ErrConflict
	}
	for id := range d.payload {
		field := access.Field(id)
		if !field.Exists || !field.Writable {
			return ErrBaseConflict
		}
	}
	rel, err := s.relation()
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `DELETE FROM `+rel+` WHERE id=$1 AND owner_user_id=$2 AND draft_version=$3`, id, access.ActorID, draftVersion)
	return err
}
func (s Store) DiscardInTx(ctx context.Context, tx pgx.Tx, access Access, id string, draftVersion int64) error {
	if err := validAccess(access); err != nil {
		return err
	}
	if !uuid.MatchString(id) || !version(draftVersion) {
		return ErrInvalid
	}
	d, err := s.owned(ctx, tx, access, id, true)
	if err != nil {
		return err
	}
	if d.DraftVersion != draftVersion {
		return ErrConflict
	}
	rel, err := s.relation()
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `DELETE FROM `+rel+` WHERE id=$1 AND owner_user_id=$2 AND draft_version=$3`, id, access.ActorID, draftVersion)
	return err
}
