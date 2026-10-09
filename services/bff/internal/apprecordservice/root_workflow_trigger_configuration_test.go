package apprecordservice

import (
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
	"reflect"
	"testing"
)

func rootTriggerCondition(field string) json.RawMessage {
	return json.RawMessage(`{"operator":"and","children":[{"fieldId":"` + field + `","operator":"eq","value":"alpha"}]}`)
}
func rootReadTriggers(t *testing.T, f recordFixture, h workflowcatalog.Head, version int64) []workflowcatalog.Trigger {
	t.Helper()
	var got []workflowcatalog.Trigger
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		var err error
		got, err = (workflowcatalog.Catalog{}).TriggersForVersionInTx(f.ctx, tx, f.app, h.FlowID, version)
		return err
	})
	return got
}
func TestRootTriggerConfigurationVersionImmutabilityAndInheritance(t *testing.T) {
	f := newRecordFixture(t)
	g := rootCatalogGraph(t, f, false)
	configs := []workflowcatalog.Trigger{{Event: "record.created", Condition: rootTriggerCondition(f.public)}}
	in := rootCatalogInput(f, recordOperationID(t, f), 0, g)
	in.Triggers = &configs
	var h workflowcatalog.Head
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		var err error
		h, err = (workflowcatalog.Catalog{}).PutVersionInTx(f.ctx, tx, in)
		return err
	})
	got := rootReadTriggers(t, f, h, 1)
	if !rootSameTriggerConfiguration(got, configs) {
		t.Fatalf("config not persisted: got=%+v want=%+v", got, configs)
	}
	h = rootCatalogEnable(t, f, rootCatalogDeploy(t, f, h))
	h2 := rootCatalogPut(t, f, h.FlowID, h.Revision, g) // older client omits new optional field
	if h2.CurrentVersion != 1 || h2.CandidateVersion != 2 {
		t.Fatalf("draft changed published version: %+v", h2)
	}
	if !rootSameTriggerConfiguration(rootReadTriggers(t, f, h2, 2), configs) {
		t.Fatal("omitted config lost existing candidate settings")
	}
	empty := []workflowcatalog.Trigger{}
	in = rootCatalogInput(f, h.FlowID, h2.Revision, g)
	in.Triggers = &empty
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		var err error
		h2, err = (workflowcatalog.Catalog{}).PutVersionInTx(f.ctx, tx, in)
		return err
	})
	if got := rootReadTriggers(t, f, h2, 3); got == nil || len(got) != 0 {
		t.Fatalf("explicit empty must clear new candidate only: %+v", got)
	}
	if !rootSameTriggerConfiguration(rootReadTriggers(t, f, h2, 1), configs) {
		t.Fatal("published config was overwritten")
	}
	if h2.CurrentVersion != 1 {
		t.Fatal("unpublished config activated")
	}
}
func TestRootTriggerConfigurationInvalidSaveRollsBack(t *testing.T) {
	f := newRecordFixture(t)
	g := rootCatalogGraph(t, f, false)
	in := rootCatalogInput(f, recordOperationID(t, f), 0, g)
	configs := []workflowcatalog.Trigger{{Event: "record.updated", Condition: rootTriggerCondition(recordOperationID(t, f))}}
	in.Triggers = &configs
	rootCatalogWantError(t, f, workflowcatalog.ErrInvalid, func(tx pgx.Tx) error { _, err := (workflowcatalog.Catalog{}).PutVersionInTx(f.ctx, tx, in); return err })
	var n int
	if err := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_definitions WHERE id=$1", in.FlowID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("invalid configuration left a flow: %d %v", n, err)
	}
}
func TestRootTriggerConfigurationCompatibleSchemaAndPublishGuard(t *testing.T) {
	f := newRecordFixture(t)
	g := rootCatalogGraph(t, f, false)
	configs := []workflowcatalog.Trigger{{Event: "record.updated", Condition: rootTriggerCondition(f.public)}}
	in := rootCatalogInput(f, recordOperationID(t, f), 0, g)
	in.Triggers = &configs
	var h workflowcatalog.Head
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		var err error
		h, err = (workflowcatalog.Catalog{}).PutVersionInTx(f.ctx, tx, in)
		return err
	})
	h = rootCatalogEnable(t, f, rootCatalogDeploy(t, f, h))
	// The graph itself has no condition or editable fields; this dependency is
	// exclusively from the configured trigger, not copied from the implementation.
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		got, err := (workflowcatalog.Catalog{}).CheckCompatibilityInTx(f.ctx, tx, f.app, f.table, []appquery.Field{{ID: f.secret, Kind: appquery.Text}, {ID: f.reference, Kind: appquery.Member}})
		if err != nil {
			return err
		}
		if len(got) != 1 || got[0].FieldID != f.public || got[0].FlowID != h.FlowID || got[0].NodeID != g.Nodes[0].ID {
			t.Fatalf("missing trigger field not reported: %+v", got)
		}
		return nil
	})
}
func TestRootTriggerConfigurationMissingVersionAndLegacyEmpty(t *testing.T) {
	f := newRecordFixture(t)
	h := rootCatalogReady(t, f, rootCatalogGraph(t, f, false))
	got := rootReadTriggers(t, f, h, 1)
	if got == nil || len(got) != 0 {
		t.Fatalf("legacy config should be []: %+v", got)
	}
	rootCatalogTx(t, f, func(tx pgx.Tx) error {
		_, err := (workflowcatalog.Catalog{}).TriggersForVersionInTx(f.ctx, tx, f.app, h.FlowID, 99)
		if !errors.Is(err, workflowcatalog.ErrMissing) {
			t.Fatalf("missing version: %v", err)
		}
		return nil
	})
}

// Conditions are JSON values; object key serialization order is not behavior.
func rootSameTriggerConfiguration(a, b []workflowcatalog.Trigger) bool {
	rawA, errA := json.Marshal(a)
	rawB, errB := json.Marshal(b)
	var left, right any
	return errA == nil && errB == nil && json.Unmarshal(rawA, &left) == nil && json.Unmarshal(rawB, &right) == nil && reflect.DeepEqual(left, right)
}
