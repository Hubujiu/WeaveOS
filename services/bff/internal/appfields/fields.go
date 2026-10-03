package appfields

import (
	"encoding/json"
	"errors"
)

var ErrInvalid = errors.New("invalid field configuration or value")

type DecimalConfig struct {
	Precision      int    `json:"precision"`
	Scale          int    `json:"scale"`
	RoundingPlaces int    `json:"roundingPlaces"`
	RoundingMode   string `json:"roundingMode"`
}
type Presentation struct {
	HelpText        *string `json:"helpText"`
	DisplayTimeZone *string `json:"displayTimeZone"`
}
type Field struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Kind         string          `json:"kind"`
	Required     bool            `json:"required"`
	Default      json.RawMessage `json:"default"`
	Config       json.RawMessage `json:"config"`
	Presentation Presentation    `json:"presentation"`
}
type Option struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}
type LayoutNode struct {
	ID       string       `json:"id"`
	Kind     string       `json:"kind"`
	FieldID  string       `json:"fieldId,omitempty"`
	Span     int          `json:"span,omitempty"`
	Title    string       `json:"title,omitempty"`
	Text     string       `json:"text,omitempty"`
	Children []LayoutNode `json:"children,omitempty"`
}

func (n LayoutNode) MarshalJSON() ([]byte, error) {
	m := map[string]any{"id": n.ID, "kind": n.Kind}
	switch n.Kind {
	case "field", "system_field":
		m["fieldId"] = n.FieldID
		if n.Span != 0 {
			m["span"] = n.Span
		}
	case "group":
		m["title"] = n.Title
		m["children"] = n.Children
		if n.Span != 0 {
			m["span"] = n.Span
		}
	case "description":
		m["text"] = n.Text
	}
	return json.Marshal(m)
}
