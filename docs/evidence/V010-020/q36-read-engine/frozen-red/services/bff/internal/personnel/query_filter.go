package personnel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

// FilterPlan contains fixed identifiers and separate bind arguments. Time args
// are absolute boundaries; callers retain them in the normalized query context.
// Canonical is the normalized public tree for its independent 16KiB limit.
type FilterPlan struct {
	FrozenDates []QueryRange
	Predicate   string
	Arguments   []any
	Canonical   json.RawMessage
}

type filterCompiler struct {
	view          string
	first, leaves int
	args          []any
}

func CompileFilter(view string, raw json.RawMessage, firstParameter int) (FilterPlan, error) {
	if view != "members" && view != "events" || firstParameter < 1 || firstParameter > 65000 {
		return FilterPlan{}, ErrInvalid
	}
	if len(raw) == 0 {
		return FilterPlan{Predicate: "TRUE", Arguments: []any{}}, nil
	}
	// Share the strict JSON Unicode validator: encoding/json alone replaces
	// unpaired escaped UTF16 surrogates, silently changing exact comparisons.
	if len(raw) > 65536 || !draftJSONUnicode(raw) {
		return FilterPlan{}, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	v, err := filterJSON(d, 0)
	if err != nil {
		return FilterPlan{}, ErrInvalid
	}
	if _, err = d.Token(); err != io.EOF {
		return FilterPlan{}, ErrInvalid
	}
	c := filterCompiler{view: view, first: firstParameter, args: []any{}}
	predicate, normalized, err := c.group(v, 1)
	if err != nil {
		return FilterPlan{}, err
	}
	var out bytes.Buffer
	e := json.NewEncoder(&out)
	e.SetEscapeHTML(false)
	if err = e.Encode(normalized); err != nil {
		return FilterPlan{}, err
	}
	canonical := bytes.TrimSuffix(out.Bytes(), []byte("\n"))
	if len(canonical) > 16384 {
		return FilterPlan{}, ErrInvalid
	}
	return FilterPlan{Predicate: predicate, Arguments: c.args, Canonical: append(json.RawMessage(nil), canonical...)}, nil
}

// Reject duplicate keys recursively before normal JSON decoding can erase them.
// The structural cap prevents pathological input before the domain depth check.
func filterJSON(d *json.Decoder, depth int) (any, error) {
	if depth > 10 {
		return nil, ErrInvalid
	}
	token, err := d.Token()
	if err != nil {
		return nil, ErrInvalid
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}
	switch delim {
	case '{':
		m := map[string]any{}
		for d.More() {
			key, err := d.Token()
			name, ok := key.(string)
			if err != nil || !ok {
				return nil, ErrInvalid
			}
			if _, exists := m[name]; exists {
				return nil, ErrInvalid
			}
			value, err := filterJSON(d, depth+1)
			if err != nil {
				return nil, err
			}
			m[name] = value
		}
		end, err := d.Token()
		if err != nil || end != json.Delim('}') {
			return nil, ErrInvalid
		}
		return m, nil
	case '[':
		a := []any{}
		for d.More() {
			v, err := filterJSON(d, depth+1)
			if err != nil {
				return nil, err
			}
			a = append(a, v)
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(']') {
			return nil, ErrInvalid
		}
		return a, nil
	default:
		return nil, ErrInvalid
	}
}
func (c *filterCompiler) bind(v any) string {
	c.args = append(c.args, v)
	return fmt.Sprintf("$%d", c.first+len(c.args)-1)
}
func (c *filterCompiler) group(v any, depth int) (string, any, error) {
	m, ok := v.(map[string]any)
	if !ok || depth > 3 || len(m) != 2 {
		return "", nil, ErrInvalid
	}
	op, ok := m["operator"].(string)
	if !ok || (op != "and" && op != "or") {
		return "", nil, ErrInvalid
	}
	children, ok := m["children"].([]any)
	if !ok || len(children) == 0 || len(children) > 20 {
		return "", nil, ErrInvalid
	}
	sql := []string{}
	normalized := []any{}
	for _, child := range children {
		node, ok := child.(map[string]any)
		if !ok {
			return "", nil, ErrInvalid
		}
		var p string
		var n any
		var err error
		if _, group := node["children"]; group {
			p, n, err = c.group(node, depth+1)
		} else {
			c.leaves++
			if c.leaves > 20 {
				return "", nil, ErrInvalid
			}
			p, n, err = c.condition(node)
		}
		if err != nil {
			return "", nil, err
		}
		sql = append(sql, p)
		normalized = append(normalized, n)
	}
	return "(" + strings.Join(sql, " "+strings.ToUpper(op)+" ") + ")", map[string]any{"operator": op, "children": normalized}, nil
}
func (c *filterCompiler) condition(m map[string]any) (string, any, error) {
	if len(m) != 3 {
		return "", nil, ErrInvalid
	}
	field, ok := m["field"].(string)
	if !ok {
		return "", nil, ErrInvalid
	}
	op, ok := m["operator"].(string)
	if !ok {
		return "", nil, ErrInvalid
	}
	value, present := m["value"]
	if !present {
		return "", nil, ErrInvalid
	}
	cols := map[string]string{"account": "q.account", "status": "q.status", "departmentIds": "q.department_ids", "identityIds": "q.identity_ids", "personnelManage": "q.personnel_manage"}
	if c.view == "events" {
		cols = map[string]string{"actorAccount": "q.actor_account", "object": "q.display_object", "detail": "q.display_detail", "action": "q.action", "outcome": "q.outcome", "occurredAt": "q.occurred_at"}
	}
	col, ok := cols[field]
	if !ok {
		return "", nil, ErrInvalid
	}
	operators := map[string]string{"eq": "=", "neq": "<>", "gt": ">", "gte": ">=", "lt": "<", "lte": "<="}
	symbol, ok := operators[op]
	if !ok || field != "occurredAt" && op != "eq" && op != "neq" {
		return "", nil, ErrInvalid
	}
	normalized := map[string]any{"field": field, "operator": op, "value": value}
	relation := field == "departmentIds" || field == "identityIds"
	if value == nil {
		if relation || op != "eq" && op != "neq" {
			return "", nil, ErrInvalid
		}
		suffix := " IS NULL"
		if op == "neq" {
			suffix = " IS NOT NULL"
		}
		return col + suffix, normalized, nil
	}
	if relation {
		id, ok := value.(string)
		if !ok || !validID(id) {
			return "", nil, ErrInvalid
		}
		id = strings.ToLower(id)
		normalized["value"] = id
		prefix := ""
		if op == "neq" {
			prefix = "NOT "
		}
		return prefix + "EXISTS (SELECT 1 FROM unnest(" + col + ") AS relation(id) WHERE relation.id=" + c.bind(id) + "::uuid)", normalized, nil
	}
	if field == "occurredAt" {
		if instant, ok := value.(string); ok {
			t, err := time.Parse(time.RFC3339Nano, instant)
			if err != nil || t.Nanosecond()%1000 != 0 {
				return "", nil, ErrInvalid
			}
			t = t.UTC()
			normalized["value"] = t.Format(time.RFC3339Nano)
			return col + " " + symbol + " " + c.bind(t) + "::timestamptz", normalized, nil
		}
		date, ok := value.(map[string]any)
		if !ok || len(date) != 2 {
			return "", nil, ErrInvalid
		}
		day, ok := date["date"].(string)
		if !ok {
			return "", nil, ErrInvalid
		}
		zone, ok := date["timeZone"].(string)
		if !ok || zone == "" || len(zone) > 128 || zone == "Local" {
			return "", nil, ErrInvalid
		}
		loc, err := time.LoadLocation(zone)
		if err != nil {
			return "", nil, ErrInvalid
		}
		start, err := time.ParseInLocation("2006-01-02", day, loc)
		if err != nil || start.Format("2006-01-02") != day {
			return "", nil, ErrInvalid
		}
		end := start.AddDate(0, 0, 1).UTC()
		start = start.UTC()
		switch op {
		case "eq":
			return "(" + col + " >= " + c.bind(start) + "::timestamptz AND " + col + " < " + c.bind(end) + "::timestamptz)", normalized, nil
		case "neq":
			return "(" + col + " < " + c.bind(start) + "::timestamptz OR " + col + " >= " + c.bind(end) + "::timestamptz)", normalized, nil
		case "gt":
			return col + " >= " + c.bind(end) + "::timestamptz", normalized, nil
		case "gte":
			return col + " >= " + c.bind(start) + "::timestamptz", normalized, nil
		case "lt":
			return col + " < " + c.bind(start) + "::timestamptz", normalized, nil
		case "lte":
			return col + " < " + c.bind(end) + "::timestamptz", normalized, nil
		}
	}
	if field == "personnelManage" {
		if _, ok := value.(bool); !ok {
			return "", nil, ErrInvalid
		}
		return col + " " + symbol + " " + c.bind(value) + "::boolean", normalized, nil
	}
	text, ok := value.(string)
	if !ok || utf8.RuneCountInString(text) > 16384 || strings.ContainsRune(text, 0) {
		return "", nil, ErrInvalid
	}
	switch field {
	case "status":
		if text != "active" && text != "disabled" {
			return "", nil, ErrInvalid
		}
	case "action":
		if !activityActions[text] {
			return "", nil, ErrInvalid
		}
	case "outcome":
		if text != "success" && text != "failure" && text != "error" {
			return "", nil, ErrInvalid
		}
	}
	return col + ` COLLATE "C" ` + symbol + " " + c.bind(text) + `::text COLLATE "C"`, normalized, nil
}
