package workflowcatalog

import (
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"reflect"
	"testing"
)

const triggerField = "00000000-0000-4000-8000-000000000001"

var triggerFields = []appquery.Field{{ID: triggerField, Kind: appquery.Text}}

func TestTriggerConfigurationNormalizedEventsAndNull(t *testing.T) {
	in := []Trigger{{Event: "record.updated"}, {Event: "manual", Condition: json.RawMessage(" null ")}, {Event: "record.created"}}
	got, err := NormalizeTriggers(in, triggerFields)
	want := []Trigger{{Event: "manual", Condition: json.RawMessage("null")}, {Event: "record.created", Condition: json.RawMessage("null")}, {Event: "record.updated", Condition: json.RawMessage("null")}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized=%+v error=%v", got, err)
	}
	if in[0].Event != "record.updated" {
		t.Fatal("input order mutated")
	}
	empty, err := NormalizeTriggers(nil, triggerFields)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("legacy configuration must normalize to [], got=%+v err=%v", empty, err)
	}
}
func TestTriggerConfigurationConditionPreservesIndependentPredicate(t *testing.T) {
	raw := json.RawMessage(`{"operator":"and","children":[{"fieldId":"` + triggerField + `","operator":"eq","value":"alpha"}]}`)
	got, err := NormalizeTriggers([]Trigger{{Event: "record.updated", Condition: raw}}, triggerFields)
	if err != nil || len(got) != 1 {
		t.Fatalf("valid condition: %+v %v", got, err)
	}
	var actual, expected any
	if json.Unmarshal(got[0].Condition, &actual) != nil || json.Unmarshal(raw, &expected) != nil || !reflect.DeepEqual(actual, expected) {
		t.Fatalf("condition changed: %s", got[0].Condition)
	}
	got[0].Condition[0] = '!'
	if raw[0] != '{' {
		t.Fatal("normalized condition aliases caller memory")
	}
}
func TestTriggerConfigurationRejectsInvalidEventsAndPredicates(t *testing.T) {
	for name, in := range map[string][]Trigger{
		"unknown_event":      {{Event: "timer"}},
		"empty_event":        {{Event: ""}},
		"duplicate":          {{Event: "manual"}, {Event: "manual"}},
		"too_many":           {{Event: "manual"}, {Event: "record.created"}, {Event: "record.updated"}, {Event: "manual"}},
		"scalar_condition":   {{Event: "manual", Condition: json.RawMessage(`true`)}},
		"unknown_field":      {{Event: "record.created", Condition: json.RawMessage(`{"operator":"and","children":[{"fieldId":"00000000-0000-4000-8000-000000000099","operator":"eq","value":"x"}]}`)}},
		"text_ordering":      {{Event: "record.created", Condition: json.RawMessage(`{"operator":"and","children":[{"fieldId":"` + triggerField + `","operator":"gt","value":"x"}]}`)}},
		"unknown_filter_key": {{Event: "manual", Condition: json.RawMessage(`{"operator":"and","children":[],"execute":"anything"}`)}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NormalizeTriggers(in, triggerFields); !errors.Is(err, ErrInvalid) {
				t.Fatalf("invalid config accepted or wrong error: %v", err)
			}
		})
	}
}
