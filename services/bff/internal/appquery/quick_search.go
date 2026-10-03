package appquery

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess"
	"github.com/jackc/pgx/v5"
)

type QuickPlan struct {
	Predicate        string
	Arguments        []any
	Canonical        json.RawMessage
	ReferencedFields []string
}

// CompileQuickSearch binds a literal ASCII-folded substring; field identifiers
// come only from the current trusted schema and are scope-checked first.
func CompileQuickSearch(raw json.RawMessage, fields []Field, policy appaccess.Policy, firstParameter int) (QuickPlan, error) {
	if firstParameter < 1 || firstParameter > 65000 || len(raw) > 16384 || len(raw) == 0 || string(raw) == "null" {
		return QuickPlan{}, ErrInvalid
	}
	v, err := parseJSON(raw)
	if err != nil {
		return QuickPlan{}, err
	}
	m, ok := v.(map[string]any)
	if !ok || len(m) != 2 {
		return QuickPlan{}, ErrInvalid
	}
	term, ok := m["term"].(string)
	if !ok {
		return QuickPlan{}, ErrInvalid
	}
	term = strings.TrimSpace(term)
	if !utf8.ValidString(term) || strings.ContainsRune(term, 0) || utf8.RuneCountInString(term) < 1 || utf8.RuneCountInString(term) > 160 {
		return QuickPlan{}, ErrInvalid
	}
	ids, ok := m["fieldIds"].([]any)
	if !ok || len(ids) < 1 || len(ids) > 20 {
		return QuickPlan{}, ErrInvalid
	}
	kinds := map[string]FieldKind{}
	for _, f := range fields {
		if kinds[f.ID] != "" {
			return QuickPlan{}, ErrInvalid
		}
		kinds[f.ID] = f.Kind
	}
	ordered := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, rawID := range ids {
		id, ok := rawID.(string)
		if !ok || !uuidPattern.MatchString(id) || seen[id] {
			return QuickPlan{}, ErrInvalid
		}
		seen[id] = true
		if !policy.CoversRead([]string{id}) {
			return QuickPlan{}, ErrForbidden
		}
		if kinds[id] != Text && kinds[id] != Multiline {
			return QuickPlan{}, ErrInvalid
		}
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	fold := []byte(term)
	for i, b := range fold {
		if b >= 'A' && b <= 'Z' {
			fold[i] = b + ('a' - 'A')
		}
	}
	term = string(fold)
	parts := make([]string, 0, len(ordered))
	for _, id := range ordered {
		col := "r." + pgx.Identifier{"f_" + strings.ReplaceAll(id, "-", "")}.Sanitize()
		parts = append(parts, fmt.Sprintf("strpos(translate(%s,'ABCDEFGHIJKLMNOPQRSTUVWXYZ','abcdefghijklmnopqrstuvwxyz'),$%d)>0", col, firstParameter))
	}
	canonical, _ := json.Marshal(struct {
		Term     string   `json:"term"`
		FieldIDs []string `json:"fieldIds"`
	}{term, ordered})
	return QuickPlan{Predicate: "(" + strings.Join(parts, " OR ") + ")", Arguments: []any{term}, Canonical: canonical, ReferencedFields: ordered}, nil
}
