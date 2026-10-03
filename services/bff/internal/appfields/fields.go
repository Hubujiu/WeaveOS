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

func RoundDecimal(string, DecimalConfig) (string, error)             { return "", ErrInvalid }
func NormalizeValue(Field, json.RawMessage) (json.RawMessage, error) { return nil, ErrInvalid }
func NormalizeFields([]Field) ([]Field, error)                       { return nil, ErrInvalid }
func NormalizeLayout([]LayoutNode, []Field) ([]LayoutNode, error)    { return nil, ErrInvalid }
