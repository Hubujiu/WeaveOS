package workflowcatalog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sort"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/flowgraph"
	"github.com/jackc/pgx/v5"
)

// Trigger belongs to an immutable definition version. It does not itself
// authorize a user or dispatch an engine command.
type Trigger struct {
	Event     string          `json:"event"`
	Condition json.RawMessage `json:"condition"`
}

func NormalizeTriggers(in []Trigger, fields []appquery.Field) ([]Trigger, error) {
	if len(in) > 3 || !validFieldContext(fields) {
		return nil, ErrInvalid
	}
	out := make([]Trigger, 0, len(in))
	seen := map[string]bool{}
	for _, trigger := range in {
		switch trigger.Event {
		case "manual", "record.created", "record.updated":
		default:
			return nil, ErrInvalid
		}
		if seen[trigger.Event] {
			return nil, ErrInvalid
		}
		seen[trigger.Event] = true
		plan, err := appquery.Compile(trigger.Condition, nil, fields, 1)
		if err != nil {
			return nil, ErrInvalid
		}
		raw := bytes.Clone(plan.Canonical)
		if len(raw) == 0 {
			raw = json.RawMessage("null")
		}
		out = append(out, Trigger{Event: trigger.Event, Condition: raw})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Event < out[j].Event })
	return out, nil
}

func decodeTriggers(raw []byte) ([]Trigger, error) {
	if len(raw) > 262144 || len(bytes.TrimSpace(raw)) == 0 || bytes.TrimSpace(raw)[0] != '[' {
		return nil, ErrNotReady
	}
	var out []Trigger
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&out) != nil || len(out) > 3 {
		return nil, ErrNotReady
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, ErrNotReady
	}
	return out, nil
}

func (Catalog) TriggersForVersionInTx(ctx context.Context, tx pgx.Tx, appID, flowID string, version int64) ([]Trigger, error) {
	if tx == nil || !validIDs(appID, flowID) || version < 1 || version > maxSafeInteger {
		return nil, ErrInvalid
	}
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT triggers_json FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2 AND version=$3`, appID, flowID, version).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrMissing
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	return decodeTriggers(raw)
}

func validateStoredTriggers(ctx context.Context, tx pgx.Tx, appID, flowID string, version int64, fields []appquery.Field) error {
	triggers, err := (Catalog{}).TriggersForVersionInTx(ctx, tx, appID, flowID, version)
	if err != nil {
		return err
	}
	if _, err = NormalizeTriggers(triggers, fields); err != nil {
		return ErrConflict
	}
	return nil
}

func triggerConflicts(flowID string, version int64, graph flowgraph.Graph, triggers []Trigger, fields []appquery.Field) []Conflict {
	var startID string
	for _, node := range graph.Nodes {
		if node.Kind == "start" {
			startID = node.ID
			break
		}
	}
	seen := map[Conflict]bool{}
	out := []Conflict{}
	for _, trigger := range triggers {
		// Reuse the same leaf/type compatibility rules as graph conditions, with
		// the real start node identifying the trigger's dependency in the API.
		synthetic := flowgraph.Graph{Nodes: []flowgraph.Node{{ID: startID, Kind: "condition", Condition: trigger.Condition}}}
		for _, conflict := range graphConflicts(flowID, version, synthetic, fields) {
			if !seen[conflict] {
				seen[conflict] = true
				out = append(out, conflict)
			}
		}
	}
	return out
}
