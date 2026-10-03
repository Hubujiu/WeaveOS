package appstructure

import (
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
)

func validateMappings(d Definition, in Input) error {
	old, next := map[string]appfields.Field{}, map[string]appfields.Field{}
	for _, f := range d.Fields {
		old[f.ID] = f
	}
	for _, f := range in.Fields {
		next[f.ID] = f
	}
	options := func(f appfields.Field) map[string]bool {
		var cfg struct {
			Options []appfields.Option `json:"options"`
		}
		json.Unmarshal(f.Config, &cfg)
		ids := map[string]bool{}
		for _, o := range cfg.Options {
			ids[o.ID] = true
		}
		return ids
	}
	for _, m := range in.OptionMappings {
		before, exists := old[m.FieldID]
		after, remaining := next[m.FieldID]
		if !exists || !remaining || before.Kind != "single_select" && before.Kind != "multi_select" || after.Kind != "single_select" && after.Kind != "multi_select" || !options(before)[m.FromOptionID] || m.ToOptionID != nil && !options(after)[*m.ToOptionID] {
			return invalid()
		}
	}
	return nil
}
