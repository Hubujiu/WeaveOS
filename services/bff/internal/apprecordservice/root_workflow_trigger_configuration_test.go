package apprecordservice

import (
	"encoding/json"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"os"
	"reflect"
	"strings"
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

func TestRootTriggerConfigurationPublishAndEnableRecheckFields(t *testing.T) {
	for _, action := range []string{"publication_snapshot", "confirm", "enable"} {
		t.Run(action, func(t *testing.T) {
			f := newRecordFixture(t)
			configs := []workflowcatalog.Trigger{{Event: "record.created", Condition: rootTriggerCondition(f.public)}}
			in := rootCatalogInput(f, recordOperationID(t, f), 0, rootCatalogGraph(t, f, false))
			in.Triggers = &configs
			var h workflowcatalog.Head
			rootCatalogTx(t, f, func(tx pgx.Tx) error {
				var err error
				h, err = (workflowcatalog.Catalog{}).PutVersionInTx(f.ctx, tx, in)
				return err
			})
			if action == "enable" {
				h = rootCatalogDeploy(t, f, h)
			}
			if _, err := f.owner.Exec(f.ctx, "UPDATE applications.logical_tables SET fields_json=fields_json-0,schema_version=schema_version+1 WHERE id=$1", f.table); err != nil {
				t.Fatal(err)
			}
			if action == "publication_snapshot" {
				rootCatalogTx(t, f, func(tx pgx.Tx) error {
					v, err := (workflowcatalog.Catalog{}).PublicationVersionInTx(f.ctx, tx, f.app, h.FlowID, 1)
					if err != nil {
						return err
					}
					if v.Compatible {
						t.Fatal("publication advertised compatibility after trigger field removal")
					}
					return nil
				})
			} else {
				rootCatalogWantError(t, f, workflowcatalog.ErrConflict, func(tx pgx.Tx) error {
					var err error
					if action == "confirm" {
						_, err = (workflowcatalog.Catalog{}).ConfirmDeploymentInTx(f.ctx, tx, f.app, h.FlowID, 1, "engine-trigger-guard")
					} else {
						_, err = (workflowcatalog.Catalog{}).EnableInTx(f.ctx, tx, f.app, h.FlowID, h.Revision)
					}
					return err
				})
			}
		})
	}
}

func TestRootTriggerConfigurationDownGuard(t *testing.T) {
	raw, err := os.ReadFile("../../../../db/migrations/00024_workflow_trigger_configuration.sql")
	if err != nil {
		t.Fatal(err)
	}
	pieces := strings.Split(string(raw), "-- +goose Down")
	if len(pieces) != 2 {
		t.Fatal("missing Down contract")
	}
	for _, nonempty := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty", true: "configured"}[nonempty], func(t *testing.T) {
			f := newRecordFixture(t)
			rootCatalogReady(t, f, rootCatalogGraph(t, f, false))
			tx, err := f.owner.Begin(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(f.ctx)
			// Private isolated database; rollback restores every row and DDL change.
			config := `[]`
			if nonempty {
				config = `[{"event":"manual","condition":null}]`
			}
			if _, err = tx.Exec(f.ctx, "UPDATE applications.workflow_versions SET triggers_json=$1", config); err != nil {
				t.Fatal(err)
			}
			_, err = tx.Exec(f.ctx, pieces[1])
			if nonempty {
				var pgErr *pgconn.PgError
				if !errors.As(err, &pgErr) || pgErr.Code != "55000" {
					t.Fatalf("configured Down must refuse without data loss, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var count int
			if err = tx.QueryRow(f.ctx, "SELECT count(*) FROM information_schema.columns WHERE table_schema='applications' AND table_name='workflow_versions' AND column_name='triggers_json'").Scan(&count); err != nil || count != 0 {
				t.Fatalf("empty configuration Down did not remove column: %d %v", count, err)
			}
		})
	}
}
