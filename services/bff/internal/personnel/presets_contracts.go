package personnel

import (
	"encoding/json"
	"time"
)

// ADR008 A1/A2. Ownership and immutable view/slot are never writable DTO fields.
type TablePreset struct {
	ID              string          `json:"id"`
	View            string          `json:"view"`
	Name            string          `json:"name"`
	Filter          json.RawMessage `json:"filter"`
	HiddenColumnIDs []string        `json:"hiddenColumnIds"`
	SchemaVersion   int             `json:"schemaVersion"`
	Version         int64           `json:"version"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}
type TablePresetList struct {
	Items []TablePreset `json:"items"`
}
type PresetCreateInput struct {
	View            string          `json:"view"`
	Name            string          `json:"name"`
	Filter          json.RawMessage `json:"filter,omitempty"`
	HiddenColumnIDs []string        `json:"hiddenColumnIds"`
	SchemaVersion   int             `json:"schemaVersion"`
}
type PresetUpdateInput struct {
	Version         int64           `json:"version"`
	Name            string          `json:"name"`
	Filter          json.RawMessage `json:"filter,omitempty"`
	HiddenColumnIDs []string        `json:"hiddenColumnIds"`
	SchemaVersion   int             `json:"schemaVersion"`
}
