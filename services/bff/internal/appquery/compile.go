// Package appquery compiles record criteria against a trusted active-field
// snapshot. The caller owns Session, live policy, RR transaction and context CAS.
package appquery

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

var ErrInvalid = errors.New("invalid record search criteria")
var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var decimalPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)

type FieldKind string

const (
	Text         FieldKind = "text"
	Multiline    FieldKind = "multiline"
	Number       FieldKind = "number"
	Money        FieldKind = "money"
	Date         FieldKind = "date"
	Datetime     FieldKind = "datetime"
	Boolean      FieldKind = "boolean"
	SingleSelect FieldKind = "single_select"
	MultiSelect  FieldKind = "multi_select"
	Member       FieldKind = "member"
	Department   FieldKind = "department"
)

type Field struct {
	ID   string
	Kind FieldKind
}
type Plan struct {
	Predicate        string
	Arguments        []any
	Order            string
	Canonical        json.RawMessage
	ReferencedFields []string
}

type compiler struct {
	fields     map[string]FieldKind
	first      int
	args       []any
	referenced []string
	seen       map[string]bool
	leaves     int
}

// Compile returns SQL fragments and bind arguments. The caller must use
// CoversRead on ReferencedFields before execution and constrain rows by the
// same live policy in the same RR snapshot.
func Compile(raw json.RawMessage, sortRaw json.RawMessage, fields []Field, firstParameter int) (Plan, error) {
	if firstParameter < 1 || firstParameter > 65000 || len(raw) > 65536 || len(sortRaw) > 16384 {
		return Plan{}, ErrInvalid
	}
	c := compiler{fields: make(map[string]FieldKind, len(fields)), first: firstParameter, seen: map[string]bool{}}
	for _, field := range fields {
		if !uuidPattern.MatchString(field.ID) || !validKind(field.Kind) || c.fields[field.ID] != "" {
			return Plan{}, ErrInvalid
		}
		c.fields[field.ID] = field.Kind
	}
	p := Plan{Predicate: "TRUE", Order: "r.created_at DESC,r.id DESC"}
	if len(bytes.TrimSpace(raw)) > 0 && string(bytes.TrimSpace(raw)) != "null" {
		v, err := parseJSON(raw)
		if err != nil {
			return Plan{}, err
		}
		var normalized any
		p.Predicate, normalized, err = c.group(v, 1)
		if err != nil {
			return Plan{}, err
		}
		p.Canonical, err = json.Marshal(normalized)
		if err != nil || len(p.Canonical) > 16384 {
			return Plan{}, ErrInvalid
		}
	}
	if len(bytes.TrimSpace(sortRaw)) > 0 && string(bytes.TrimSpace(sortRaw)) != "null" {
		v, err := parseJSON(sortRaw)
		if err != nil {
			return Plan{}, err
		}
		m, ok := v.(map[string]any)
		if !ok || len(m) != 2 {
			return Plan{}, ErrInvalid
		}
		field, ok := m["fieldId"].(string)
		if !ok {
			return Plan{}, ErrInvalid
		}
		direction, ok := m["direction"].(string)
		if !ok || direction != "asc" && direction != "desc" {
			return Plan{}, ErrInvalid
		}
		kind, col, err := c.column(field)
		if err != nil || !sortable(kind) {
			return Plan{}, ErrInvalid
		}
		c.reference(field)
		d := strings.ToUpper(direction)
		p.Order = col + " " + d + " NULLS LAST,r.id " + d
	}
	p.Arguments = c.args
	p.ReferencedFields = c.referenced
	return p, nil
}

func validKind(k FieldKind) bool {
	switch k {
	case Text, Multiline, Number, Money, Date, Datetime, Boolean, SingleSelect, MultiSelect, Member, Department:
		return true
	}
	return false
}
func sortable(k FieldKind) bool { return k == Number || k == Money || k == Date || k == Datetime }

func parseJSON(raw []byte) (any, error) {
	if !validUnicode(raw) {
		return nil, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	v, err := node(d, 0)
	if err != nil {
		return nil, err
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, ErrInvalid
	}
	return v, nil
}

// encoding/json replaces unpaired UTF-16 surrogates. Exact predicates must
// reject malformed strings rather than silently compare a changed value.
func validUnicode(raw []byte) bool {
	if !utf8.Valid(raw) || !json.Valid(raw) {
		return false
	}
	inString := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			inString = !inString
			continue
		}
		if !inString || raw[i] != '\\' {
			continue
		}
		i++
		if raw[i] != 'u' {
			continue
		}
		n, e := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if e != nil || n == 0 {
			return false
		}
		i += 4
		if n >= 0xd800 && n <= 0xdbff {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return false
			}
			low, e := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
			if e != nil || low < 0xdc00 || low > 0xdfff || utf16.DecodeRune(rune(n), rune(low)) == utf8.RuneError {
				return false
			}
			i += 6
		} else if n >= 0xdc00 && n <= 0xdfff {
			return false
		}
	}
	return true
}

// node rejects duplicate keys before encoding/json can discard them.
func node(d *json.Decoder, depth int) (any, error) {
	if depth > 12 {
		return nil, ErrInvalid
	}
	tok, err := d.Token()
	if err != nil {
		return nil, ErrInvalid
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return tok, nil
	}
	switch delim {
	case '{':
		m := map[string]any{}
		for d.More() {
			k, e := d.Token()
			key, ok := k.(string)
			if e != nil || !ok {
				return nil, ErrInvalid
			}
			if _, exists := m[key]; exists {
				return nil, ErrInvalid
			}
			v, e := node(d, depth+1)
			if e != nil {
				return nil, e
			}
			m[key] = v
		}
		end, e := d.Token()
		if e != nil || end != json.Delim('}') {
			return nil, ErrInvalid
		}
		return m, nil
	case '[':
		a := []any{}
		for d.More() {
			v, e := node(d, depth+1)
			if e != nil {
				return nil, e
			}
			a = append(a, v)
		}
		end, e := d.Token()
		if e != nil || end != json.Delim(']') {
			return nil, ErrInvalid
		}
		return a, nil
	default:
		return nil, ErrInvalid
	}
}

func (c *compiler) column(id string) (FieldKind, string, error) {
	switch id {
	case "createdAt":
		return Datetime, "r.created_at", nil
	case "updatedAt":
		return Datetime, "r.updated_at", nil
	}
	k, ok := c.fields[id]
	if !ok || !uuidPattern.MatchString(id) {
		return "", "", ErrInvalid
	}
	return k, "r." + pgx.Identifier{"f_" + strings.ReplaceAll(id, "-", "")}.Sanitize(), nil
}
func (c *compiler) reference(id string) {
	if id == "createdAt" || id == "updatedAt" || c.seen[id] {
		return
	}
	c.seen[id] = true
	c.referenced = append(c.referenced, id)
}
func (c *compiler) bind(v any) string {
	c.args = append(c.args, v)
	return fmt.Sprintf("$%d", c.first+len(c.args)-1)
}

func (c *compiler) group(v any, depth int) (string, any, error) {
	m, ok := v.(map[string]any)
	if !ok || depth > 3 || len(m) != 2 {
		return "", nil, ErrInvalid
	}
	op, ok := m["operator"].(string)
	if !ok || op != "and" && op != "or" {
		return "", nil, ErrInvalid
	}
	children, ok := m["children"].([]any)
	if !ok || len(children) == 0 || len(children) > 20 {
		return "", nil, ErrInvalid
	}
	parts := make([]string, 0, len(children))
	normalized := make([]any, 0, len(children))
	for _, child := range children {
		m, ok := child.(map[string]any)
		if !ok {
			return "", nil, ErrInvalid
		}
		var sql string
		var n any
		var err error
		if _, group := m["children"]; group {
			sql, n, err = c.group(m, depth+1)
		} else {
			c.leaves++
			if c.leaves > 20 {
				return "", nil, ErrInvalid
			}
			sql, n, err = c.condition(m)
		}
		if err != nil {
			return "", nil, err
		}
		parts = append(parts, sql)
		normalized = append(normalized, n)
	}
	return "(" + strings.Join(parts, " "+strings.ToUpper(op)+" ") + ")", map[string]any{"operator": op, "children": normalized}, nil
}
func (c *compiler) condition(m map[string]any) (string, any, error) {
	if len(m) != 3 {
		return "", nil, ErrInvalid
	}
	id, ok := m["fieldId"].(string)
	if !ok {
		return "", nil, ErrInvalid
	}
	op, ok := m["operator"].(string)
	if !ok {
		return "", nil, ErrInvalid
	}
	value, ok := m["value"]
	if !ok {
		return "", nil, ErrInvalid
	}
	kind, col, err := c.column(id)
	if err != nil {
		return "", nil, err
	}
	if op != "eq" && op != "neq" && !(sortable(kind) && (op == "gt" || op == "gte" || op == "lt" || op == "lte")) {
		return "", nil, ErrInvalid
	}
	c.reference(id)
	if value == nil {
		if op != "eq" && op != "neq" {
			return "", nil, ErrInvalid
		}
		if op == "eq" {
			return col + " IS NULL", m, nil
		}
		return col + " IS NOT NULL", m, nil
	}
	var suffix string
	var normalized any = value
	switch kind {
	case Text, Multiline:
		s, ok := value.(string)
		if !ok {
			return "", nil, ErrInvalid
		}
		normalized = s
		suffix = ""
	case Number, Money:
		s, ok := value.(string)
		if !ok || !decimalPattern.MatchString(s) || len(s) > 128 {
			return "", nil, ErrInvalid
		}
		suffix = "::numeric"
	case Date:
		s, ok := value.(string)
		if !ok || len(s) != 10 {
			return "", nil, ErrInvalid
		}
		d, e := time.Parse("2006-01-02", s)
		if e != nil || d.Format("2006-01-02") != s {
			return "", nil, ErrInvalid
		}
		suffix = "::date"
	case Datetime:
		s, ok := value.(string)
		if !ok {
			return "", nil, ErrInvalid
		}
		dt, e := time.Parse(time.RFC3339Nano, s)
		if e != nil || !strings.ContainsAny(s[10:], "Z+-") {
			return "", nil, ErrInvalid
		}
		normalized = dt.UTC().Format(time.RFC3339Nano)
		suffix = "::timestamptz"
	case Boolean:
		if _, ok := value.(bool); !ok {
			return "", nil, ErrInvalid
		}
		suffix = "::boolean"
	case SingleSelect, Member, Department:
		s, ok := value.(string)
		if !ok || !uuidPattern.MatchString(s) {
			return "", nil, ErrInvalid
		}
		suffix = "::uuid"
	case MultiSelect:
		a, ok := value.([]any)
		if !ok {
			return "", nil, ErrInvalid
		}
		set := map[string]bool{}
		for _, v := range a {
			s, ok := v.(string)
			if !ok || !uuidPattern.MatchString(s) {
				return "", nil, ErrInvalid
			}
			set[s] = true
		}
		ids := make([]string, 0, len(set))
		for id := range set {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		if len(ids) == 0 {
			m["value"] = nil
			if op == "eq" {
				return col + " IS NULL", m, nil
			}
			return col + " IS NOT NULL", m, nil
		}
		m["value"] = ids
		arg := c.bind(ids) + "::uuid[]"
		eq := "(" + col + " @> " + arg + " AND " + col + " <@ " + arg + ")"
		if op == "eq" {
			return eq, m, nil
		}
		return "(" + col + " IS NOT NULL AND NOT " + eq + ")", m, nil
	default:
		return "", nil, ErrInvalid
	}
	m["value"] = normalized
	symbol := map[string]string{"eq": "=", "neq": "<>", "gt": ">", "gte": ">=", "lt": "<", "lte": "<="}[op]
	if symbol == "" {
		return "", nil, ErrInvalid
	}
	return col + " " + symbol + " " + c.bind(normalized) + suffix, m, nil
}
