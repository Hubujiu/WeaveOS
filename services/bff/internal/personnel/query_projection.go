package personnel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"hash"
	"sort"
	"strings"
	"time"
)

type ProjectionLabel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type MemberProjection struct {
	ID              string            `json:"id"`
	CreatedAt       time.Time         `json:"createdAt"`
	Account         string            `json:"account"`
	Status          string            `json:"status"`
	BootstrapAdmin  bool              `json:"bootstrapAdmin"`
	PersonnelManage bool              `json:"personnelManage"`
	Departments     []ProjectionLabel `json:"departments"`
	Identities      []ProjectionLabel `json:"identities"`
}
type MemberProjectionPage struct {
	Items       []MemberProjection
	Total       int64
	Fingerprint string
}

// No HTTP integration yet. The engine uses scanMemberProjection inside the SAME
// authorized RR transaction for old-baseline validation and new query reads.
func (a *Application) MemberProjection(ctx context.Context, p session.Principal, input MemberQueryInput) (MemberProjectionPage, error) {
	tx, err := a.read(ctx, p)
	if err != nil {
		return MemberProjectionPage{}, err
	}
	defer tx.Rollback(context.Background())
	return scanMemberProjection(ctx, tx, input)
}

func memberProjectionQuery(input MemberQueryInput) (string, []any, PageQuery, error) {
	return memberSelectQuery(input, false)
}

func memberProjectionFilter(input MemberQueryInput) (string, []any, PageQuery, error) {
	page, err := normalizedPage(input.PageQuery)
	if err != nil {
		return "", nil, page, err
	}
	if input.DepartmentID != "" && !validID(input.DepartmentID) || input.IdentityID != "" && !validID(input.IdentityID) {
		return "", nil, page, ErrInvalid
	}
	var raw []byte
	if input.Filter != nil {
		raw, err = projectionJSON(input.Filter)
		if err != nil {
			return "", nil, page, err
		}
	}
	plan, err := CompileFilter("members", raw, 4)
	if err != nil {
		return "", nil, page, err
	}
	args := append([]any{page.Search, input.DepartmentID, input.IdentityID}, plan.Arguments...)
	return plan.Predicate, args, page, nil
}
func projectionJSON(v any) ([]byte, error) {
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(out.Bytes(), []byte("\n")), nil
}
func fingerprintFrame(h hash.Hash, v any) error {
	encoded, err := projectionJSON(v)
	if err != nil {
		return err
	}
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(encoded)))
	_, _ = h.Write(size[:])
	_, _ = h.Write(encoded)
	return nil
}
func scanMemberProjection(ctx context.Context, tx pgx.Tx, input MemberQueryInput) (MemberProjectionPage, error) {
	sql, args, page, err := memberProjectionQuery(input)
	if err != nil {
		return MemberProjectionPage{}, err
	}
	h := sha256.New()
	_, _ = h.Write([]byte("weaveos:q36:member-projection:v1\x00"))
	if err = hashMemberFilterReferences(ctx, tx, input, h); err != nil {
		return MemberProjectionPage{}, err
	}
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return MemberProjectionPage{}, err
	}
	defer rows.Close()
	result := MemberProjectionPage{Items: []MemberProjection{}}
	offset := int64(page.Page-1) * int64(page.PageSize)
	for rows.Next() {
		row, scanErr := readMemberProjection(rows)
		if scanErr != nil {
			return MemberProjectionPage{}, scanErr
		}
		if err = fingerprintFrame(h, row); err != nil {
			return MemberProjectionPage{}, err
		}
		if result.Total >= offset && result.Total < offset+int64(page.PageSize) {
			result.Items = append(result.Items, row)
		}
		result.Total++
	}
	if err = rows.Err(); err != nil {
		return MemberProjectionPage{}, err
	}
	if err = fingerprintFrame(h, result.Total); err != nil {
		return MemberProjectionPage{}, err
	}
	result.Fingerprint = hex.EncodeToString(h.Sum(nil))
	return result, nil
}

// Selected relation labels/existence are relevant even when the result is empty.
// Reference count is bounded by the validated <=20 leaves plus two shortcuts.
func hashMemberFilterReferences(ctx context.Context, tx pgx.Tx, input MemberQueryInput, h hash.Hash) error {
	refs := map[string]string{}
	add := func(kind, id string) {
		if id != "" {
			refs[kind+":"+strings.ToLower(id)] = kind
		}
	}
	add("department", input.DepartmentID)
	add("identity", input.IdentityID)
	var visit func(json.RawMessage)
	visit = func(raw json.RawMessage) {
		var node struct {
			Field    string            `json:"field"`
			Value    json.RawMessage   `json:"value"`
			Children []json.RawMessage `json:"children"`
		}
		_ = json.Unmarshal(raw, &node)
		if node.Field == "identityIds" || node.Field == "departmentIds" {
			var id string
			_ = json.Unmarshal(node.Value, &id)
			kind := "identity"
			if node.Field == "departmentIds" {
				kind = "department"
			}
			add(kind, id)
		}
		for _, child := range node.Children {
			visit(child)
		}
	}
	if input.Filter != nil {
		for _, child := range input.Filter.Children {
			visit(child)
		}
	}
	keys := make([]string, 0, len(refs))
	for key := range refs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return fingerprintFrame(h, []any{})
	}
	values := []string{}
	args := []any{}
	for _, key := range keys {
		kind := refs[key]
		id := strings.TrimPrefix(key, kind+":")
		args = append(args, kind, id)
		values = append(values, fmt.Sprintf("($%d::text,$%d::uuid)", len(args)-1, len(args)))
	}
	sql := `WITH requested(kind,id) AS (VALUES ` + strings.Join(values, ",") + `) SELECT r.kind,r.id::text,CASE WHEN r.kind='identity' THEN i.name ELSE d.name END FROM requested r LEFT JOIN personnel.identities i ON r.kind='identity' AND i.id=r.id LEFT JOIN personnel.departments d ON r.kind='department' AND d.id=r.id ORDER BY r.kind,r.id`
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var kind, id string
		var name *string
		if err = rows.Scan(&kind, &id, &name); err != nil {
			return err
		}
		if err = fingerprintFrame(h, []any{kind, id, name}); err != nil {
			return err
		}
	}
	return rows.Err()
}
