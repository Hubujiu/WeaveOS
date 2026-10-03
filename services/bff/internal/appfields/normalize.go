package appfields

import (
	"bytes"
	"encoding/json"
	"math/big"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var decimalPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)

func ValidID(s string) bool { return uuidPattern.MatchString(s) }
func decimalValid(c DecimalConfig) bool {
	return c.Precision >= 1 && c.Precision <= 38 && c.Scale >= 0 && c.Scale <= 18 && c.Scale <= c.Precision && c.RoundingPlaces >= -18 && c.RoundingPlaces <= 18 && c.RoundingPlaces <= c.Scale && (c.RoundingMode == "HALF_UP" || c.RoundingMode == "HALF_EVEN" || c.RoundingMode == "TOWARD_ZERO" || c.RoundingMode == "FLOOR" || c.RoundingMode == "CEILING")
}
func pow10(n int) *big.Int { return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil) }

// Exact arithmetic over decimal coefficients. No float conversion or implicit PG rounding.
func RoundDecimal(s string, c DecimalConfig) (string, error) {
	if !decimalValid(c) || !decimalPattern.MatchString(s) {
		return "", ErrInvalid
	}
	negative := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	parts := strings.Split(s, ".")
	fraction := 0
	if len(parts) == 2 {
		fraction = len(parts[1])
	}
	n, _ := new(big.Int).SetString(strings.Join(parts, ""), 10)
	places := fraction - c.RoundingPlaces
	if places > 0 {
		div := pow10(places)
		q, rem := new(big.Int), new(big.Int)
		q.QuoRem(n, div, rem)
		up := false
		if rem.Sign() != 0 {
			switch c.RoundingMode {
			case "HALF_UP":
				up = new(big.Int).Lsh(new(big.Int).Set(rem), 1).Cmp(div) >= 0
			case "HALF_EVEN":
				cmp := new(big.Int).Lsh(new(big.Int).Set(rem), 1).Cmp(div)
				up = cmp > 0 || cmp == 0 && q.Bit(0) == 1
			case "FLOOR":
				up = negative
			case "CEILING":
				up = !negative
			}
		}
		if up {
			q.Add(q, big.NewInt(1))
		}
		n = q
	} else {
		n.Mul(n, pow10(-places))
	}
	n.Mul(n, pow10(c.Scale-c.RoundingPlaces))
	digits := n.String()
	if len(digits) > c.Precision {
		return "", ErrInvalid
	}
	if c.Scale > 0 {
		if len(digits) <= c.Scale {
			digits = strings.Repeat("0", c.Scale-len(digits)+1) + digits
		}
		i := len(digits) - c.Scale
		digits = digits[:i] + "." + digits[i:]
	}
	if negative && n.Sign() != 0 {
		digits = "-" + digits
	}
	return digits, nil
}
func Strict(raw json.RawMessage, target any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(target) != nil {
		return ErrInvalid
	}
	return nil
}
func object(raw json.RawMessage, keys ...string) (map[string]json.RawMessage, error) {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil || m == nil {
		return nil, ErrInvalid
	}
	for k, v := range m {
		known := false
		for _, key := range keys {
			known = known || k == key
		}
		if !known || bytes.Equal(v, []byte("null")) {
			return nil, ErrInvalid
		}
	}
	return m, nil
}
func normalizeConfig(f Field) (Field, error) {
	var c any
	switch f.Kind {
	case "number", "money":
		m, e := object(f.Config, "precision", "scale", "roundingPlaces", "roundingMode")
		if e != nil {
			return f, e
		}
		v := DecimalConfig{Precision: 38, RoundingMode: "HALF_UP"}
		if f.Kind == "money" {
			v.Scale = 2
		}
		for k, b := range m {
			e = nil
			switch k {
			case "precision":
				e = json.Unmarshal(b, &v.Precision)
			case "scale":
				e = json.Unmarshal(b, &v.Scale)
			case "roundingMode":
				e = json.Unmarshal(b, &v.RoundingMode)
			}
			if e != nil {
				return f, ErrInvalid
			}
		}
		if e != nil {
			return f, ErrInvalid
		}
		v.RoundingPlaces = v.Scale
		if b, ok := m["roundingPlaces"]; ok {
			if json.Unmarshal(b, &v.RoundingPlaces) != nil {
				return f, ErrInvalid
			}
		}
		if !decimalValid(v) {
			return f, ErrInvalid
		}
		c = v
	case "text", "multiline":
		var keys map[string]json.RawMessage
		if json.Unmarshal(f.Config, &keys) != nil || len(keys) != 1 {
			return f, ErrInvalid
		}
		if _, ok := keys["maxLength"]; !ok {
			return f, ErrInvalid
		}
		var v struct {
			MaxLength *int `json:"maxLength"`
		}
		if Strict(f.Config, &v) != nil || string(f.Config) == "null" || v.MaxLength != nil && *v.MaxLength < 1 {
			return f, ErrInvalid
		}
		c = v
	case "datetime":
		m, e := object(f.Config, "precision")
		if e != nil {
			return f, e
		}
		v := struct {
			Precision string `json:"precision"`
		}{"second"}
		if b, ok := m["precision"]; ok && json.Unmarshal(b, &v.Precision) != nil {
			return f, ErrInvalid
		}
		if v.Precision != "minute" && v.Precision != "second" && v.Precision != "millisecond" {
			return f, ErrInvalid
		}
		c = v
	case "single_select", "multi_select":
		var v struct {
			Options []Option `json:"options"`
		}
		if Strict(f.Config, &v) != nil || v.Options == nil {
			return f, ErrInvalid
		}
		seen := map[string]bool{}
		for i, o := range v.Options {
			o.Label = strings.TrimSpace(o.Label)
			if !ValidID(o.ID) || seen[o.ID] || !validName(o.Label) {
				return f, ErrInvalid
			}
			seen[o.ID] = true
			v.Options[i] = o
		}
		c = v
	case "date", "boolean", "member", "department":
		if _, e := object(f.Config); e != nil {
			return f, e
		}
		c = struct{}{}
	default:
		return f, ErrInvalid
	}
	if f.Kind == "datetime" {
		zone := "UTC"
		if f.Presentation.DisplayTimeZone != nil && *f.Presentation.DisplayTimeZone != "" {
			zone = *f.Presentation.DisplayTimeZone
		}
		if _, e := time.LoadLocation(zone); e != nil {
			return f, ErrInvalid
		}
		f.Presentation.DisplayTimeZone = &zone
	} else {
		if f.Presentation.DisplayTimeZone != nil && *f.Presentation.DisplayTimeZone != "" {
			return f, ErrInvalid
		}
		f.Presentation.DisplayTimeZone = nil
	}
	f.Config, _ = json.Marshal(c)
	return f, nil
}
func validName(s string) bool {
	return s != "" && utf8.ValidString(s) && utf8.RuneCountInString(s) <= 100 && !strings.ContainsRune(s, 0)
}
func NormalizeFields(fields []Field) ([]Field, error) {
	if fields == nil {
		return nil, ErrInvalid
	}
	out := make([]Field, 0, len(fields))
	ids := map[string]bool{}
	for _, f := range fields {
		if !ValidID(f.ID) || ids[f.ID] {
			return nil, ErrInvalid
		}
		ids[f.ID] = true
		f.Name = strings.TrimSpace(f.Name)
		if !validName(f.Name) {
			return nil, ErrInvalid
		}
		var e error
		f, e = normalizeConfig(f)
		if e != nil {
			return nil, e
		}
		if len(f.Default) == 0 {
			return nil, ErrInvalid
		}
		if string(f.Default) != "null" {
			f.Default, e = NormalizeValue(f, f.Default)
			if e != nil {
				return nil, e
			}
		}
		out = append(out, f)
	}
	return out, nil
}
func NormalizeValue(f Field, input json.RawMessage) (json.RawMessage, error) {
	if string(input) == "null" {
		if f.Required {
			return nil, ErrInvalid
		}
		return json.RawMessage("null"), nil
	}
	var value any
	switch f.Kind {
	case "number", "money":
		var s string
		var c DecimalConfig
		if json.Unmarshal(input, &s) != nil || json.Unmarshal(f.Config, &c) != nil {
			return nil, ErrInvalid
		}
		s, e := RoundDecimal(s, c)
		if e != nil {
			return nil, e
		}
		value = s
	case "text", "multiline":
		var s string
		var c struct {
			MaxLength *int `json:"maxLength"`
		}
		if json.Unmarshal(input, &s) != nil || json.Unmarshal(f.Config, &c) != nil || !utf8.ValidString(s) || strings.ContainsRune(s, 0) || c.MaxLength != nil && utf8.RuneCountInString(s) > *c.MaxLength {
			return nil, ErrInvalid
		}
		value = s
	case "boolean":
		var b bool
		if json.Unmarshal(input, &b) != nil {
			return nil, ErrInvalid
		}
		value = b
	case "member", "department":
		var s string
		if json.Unmarshal(input, &s) != nil || !ValidID(s) {
			return nil, ErrInvalid
		}
		value = s
	case "date", "datetime":
		var s string
		if json.Unmarshal(input, &s) != nil {
			return nil, ErrInvalid
		}
		if f.Kind == "date" {
			t, e := time.Parse("2006-01-02", s)
			if e != nil || len(s) != 10 || t.Format("2006-01-02") != s {
				return nil, ErrInvalid
			}
			value = s
		} else {
			t, e := time.Parse(time.RFC3339Nano, s)
			if e != nil {
				return nil, ErrInvalid
			}
			var c struct {
				Precision string `json:"precision"`
			}
			if json.Unmarshal(f.Config, &c) != nil {
				return nil, ErrInvalid
			}
			if c.Precision == "" {
				c.Precision = "second"
			}
			unit := time.Second
			switch c.Precision {
			case "minute":
				unit = time.Minute
			case "millisecond":
				unit = time.Millisecond
			case "second":
			default:
				return nil, ErrInvalid
			}
			t = t.UTC().Truncate(unit)
			if c.Precision == "millisecond" {
				value = t.Format("2006-01-02T15:04:05.000Z")
			} else {
				value = t.Format(time.RFC3339)
			}
		}
	case "single_select", "multi_select":
		var c struct {
			Options []Option `json:"options"`
		}
		if json.Unmarshal(f.Config, &c) != nil {
			return nil, ErrInvalid
		}
		var ids []string
		if f.Kind == "single_select" {
			var s string
			if json.Unmarshal(input, &s) != nil {
				return nil, ErrInvalid
			}
			ids = []string{s}
		} else if json.Unmarshal(input, &ids) != nil || ids == nil {
			return nil, ErrInvalid
		}
		seen := map[string]bool{}
		for _, id := range ids {
			seen[id] = true
		}
		ordered := []string{}
		for _, o := range c.Options {
			if seen[o.ID] {
				ordered = append(ordered, o.ID)
				delete(seen, o.ID)
			}
		}
		if len(seen) > 0 {
			return nil, ErrInvalid
		}
		if len(ordered) == 0 {
			if f.Required {
				return nil, ErrInvalid
			}
			return json.RawMessage("null"), nil
		}
		if f.Kind == "single_select" {
			value = ordered[0]
		} else {
			value = ordered
		}
	default:
		return nil, ErrInvalid
	}
	b, e := json.Marshal(value)
	return b, e
}
func NormalizeLayout(nodes []LayoutNode, fields []Field) ([]LayoutNode, error) {
	if nodes == nil {
		return nil, ErrInvalid
	}
	ids := map[string]bool{}
	fieldIDs := map[string]bool{}
	for _, f := range fields {
		fieldIDs[f.ID] = true
	}
	used := map[string]bool{}
	var walk func([]LayoutNode) ([]LayoutNode, error)
	walk = func(ns []LayoutNode) ([]LayoutNode, error) {
		out := make([]LayoutNode, 0, len(ns))
		for _, n := range ns {
			if !ValidID(n.ID) || ids[n.ID] {
				return nil, ErrInvalid
			}
			ids[n.ID] = true
			switch n.Kind {
			case "field", "system_field", "group":
				if n.Span == 0 {
					n.Span = 12
				}
				if n.Span < 1 || n.Span > 12 {
					return nil, ErrInvalid
				}
				if n.Kind == "group" {
					if n.Children == nil {
						return nil, ErrInvalid
					}
					var e error
					n.Children, e = walk(n.Children)
					if e != nil {
						return nil, e
					}
				} else {
					valid := fieldIDs[n.FieldID]
					if n.Kind == "system_field" {
						valid = n.FieldID == "id" || n.FieldID == "createdBy" || n.FieldID == "createdAt" || n.FieldID == "updatedAt" || n.FieldID == "recordVersion"
					}
					if !valid || used[n.FieldID] || n.Children != nil {
						return nil, ErrInvalid
					}
					used[n.FieldID] = true
				}
			case "divider", "description":
				if n.Span != 0 || n.Children != nil || n.FieldID != "" {
					return nil, ErrInvalid
				}
			default:
				return nil, ErrInvalid
			}
			out = append(out, n)
		}
		return out, nil
	}
	return walk(nodes)
}
