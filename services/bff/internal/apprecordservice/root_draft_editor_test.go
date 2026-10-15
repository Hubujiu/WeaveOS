package apprecordservice

import (
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appdrafts"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/jackc/pgx/v5"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func rootDraftEditorCreate(t *testing.T, f recordFixture, target *string) appdrafts.Draft {
	t.Helper()
	var op string
	if err := f.owner.QueryRow(f.ctx, "SELECT gen_random_uuid()::text").Scan(&op); err != nil {
		t.Fatal(err)
	}
	var base *int64
	if target != nil {
		v := int64(1)
		base = &v
	}
	d, err := f.service.CreateDraft(f.ctx, f.principal, DraftCreateRequest{AppID: f.app, ViewID: f.view, OperationID: op, SchemaVersion: 1, TargetRecordID: target, BaseRecordVersion: base, Values: appdrafts.Values{f.public: "unfinished"}}, applications.Metadata{RequestID: "root-draft-editor"})
	if err != nil {
		t.Fatal(err)
	}
	return d
}
func rootDraftEditorRevoke(t *testing.T, f recordFixture, action string) {
	t.Helper()
	if _, err := f.owner.Exec(f.ctx, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND grant_id IN (SELECT id FROM applications.grants WHERE app_id=$1 AND action=$2)", f.app, action); err != nil {
		t.Fatal(err)
	}
	if _, err := f.owner.Exec(f.ctx, "DELETE FROM applications.grants WHERE app_id=$1 AND action=$2", f.app, action); err != nil {
		t.Fatal(err)
	}
}
func rootDraftEditorSecretOwn(t *testing.T, f recordFixture) {
	t.Helper()
	var grant string
	if err := f.owner.QueryRow(f.ctx, "INSERT INTO applications.grants(app_id,group_id,resource_kind,resource_id,action,row_scope) SELECT app_id,group_id,'form',resource_id,'data.edit','own' FROM applications.grants WHERE app_id=$1 AND action='data.edit' LIMIT 1 RETURNING id::text", f.app).Scan(&grant); err != nil {
		t.Fatal(err)
	}
	if _, err := f.owner.Exec(f.ctx, "INSERT INTO applications.grant_fields(app_id,grant_id,field_id,table_id) VALUES($1,$2,$3,$4)", f.app, grant, f.secret, f.table); err != nil {
		t.Fatal(err)
	}
}
func TestRootDraftEditorOwnEditDoesNotRequireRecordRead(t *testing.T) {
	f := newRecordFixture(t)
	d := rootDraftEditorCreate(t, f, &f.ownRecord)
	rootDraftEditorRevoke(t, f, "data.read")
	rootDraftEditorRevoke(t, f, "data.create")
	if _, err := f.owner.Exec(f.ctx, "UPDATE applications.grants SET row_scope='own' WHERE app_id=$1 AND action='data.edit'", f.app); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.GetRecord(f.ctx, f.principal, f.app, f.view, f.ownRecord); err == nil {
		t.Fatal("fixture must deny ordinary record read")
	}
	got, err := f.service.GetDraftEditorContext(f.ctx, f.principal, f.app, f.view, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{f.public, f.reference}
	sort.Strings(want)
	sort.Strings(got.EditableFieldIDs)
	if !reflect.DeepEqual(got.EditableFieldIDs, want) || got.RecordVersion == nil || *got.RecordVersion != 1 || got.Draft.ID != d.ID || !got.CanCommit || got.AppID != f.app || got.SchemaVersion != 1 || got.ViewVersion != 1 {
		t.Fatalf("current edit projection: %+v", got)
	}
	raw, _ := json.Marshal(got)
	var wire map[string]json.RawMessage
	if json.Unmarshal(raw, &wire) != nil || len(wire) != 8 || strings.Contains(string(raw), "private alpha") || strings.Contains(string(raw), "createdBy") {
		t.Fatalf("unexpected wire %s", raw)
	}
}
func TestRootDraftEditorMixedOwnMaskUsesTargetOwnerNotDraftOwner(t *testing.T) {
	f := newRecordFixture(t)
	own := rootDraftEditorCreate(t, f, &f.ownRecord)
	other := rootDraftEditorCreate(t, f, &f.otherRecord)
	rootDraftEditorSecretOwn(t, f)
	rootDraftEditorRevoke(t, f, "data.read")
	for _, tc := range []struct {
		name, id string
		secret   bool
	}{{"own", own.ID, true}, {"foreign", other.ID, false}} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := f.service.GetDraftEditorContext(f.ctx, f.principal, f.app, f.view, tc.id)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, id := range got.EditableFieldIDs {
				if id == f.secret {
					found = true
				}
			}
			if found != tc.secret {
				t.Fatalf("target-specific missing-key authority %+v", got)
			}
			if _, ok := got.Draft.Values[f.secret]; ok {
				t.Fatal("record value borrowed into draft")
			}
		})
	}
}
func TestRootDraftEditorNewUsesCurrentCreateMaskAndNoRecordVersion(t *testing.T) {
	f := newRecordFixture(t)
	d := rootDraftEditorCreate(t, f, nil)
	got, err := f.service.GetDraftEditorContext(f.ctx, f.principal, f.app, f.view, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RecordVersion != nil || !got.CanCommit || got.Draft.TargetRecordID != nil || len(got.EditableFieldIDs) != 2 {
		t.Fatalf("new context %+v", got)
	}
}
func TestRootDraftEditorChangedBaseKeepsOriginalDraftAndCannotCommit(t *testing.T) {
	f := newRecordFixture(t)
	d := rootDraftEditorCreate(t, f, &f.ownRecord)
	table := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	if _, err := f.owner.Exec(f.ctx, "UPDATE "+table+" SET record_version=2 WHERE id=$1", f.ownRecord); err != nil {
		t.Fatal(err)
	}
	got, err := f.service.GetDraftEditorContext(f.ctx, f.principal, f.app, f.view, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RecordVersion == nil || *got.RecordVersion != 2 || got.Draft.BaseRecordVersion == nil || *got.Draft.BaseRecordVersion != 1 || got.CanCommit || !got.Draft.HasConflicts {
		t.Fatalf("must preserve old binding %+v", got)
	}
}
func TestRootDraftEditorCurrentRevokedFieldNeverLeaksStoredValue(t *testing.T) {
	f := newRecordFixture(t)
	d := rootDraftEditorCreate(t, f, nil)
	if _, err := f.owner.Exec(f.ctx, "DELETE FROM applications.grant_fields WHERE app_id=$1 AND field_id=$2 AND grant_id IN (SELECT id FROM applications.grants WHERE app_id=$1 AND action='data.create')", f.app, f.public); err != nil {
		t.Fatal(err)
	}
	got, err := f.service.GetDraftEditorContext(f.ctx, f.principal, f.app, f.view, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CanCommit || !got.Draft.HasConflicts {
		t.Fatal("missing conflict")
	}
	for _, id := range got.EditableFieldIDs {
		if id == f.public {
			t.Fatal("revoked field writable")
		}
	}
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "unfinished") {
		t.Fatal("revoked value leaked")
	}
}
func TestRootDraftEditorRejectsRevokedMenuAndForeignOwner(t *testing.T) {
	for _, mode := range []string{"menu", "actor", "view"} {
		t.Run(mode, func(t *testing.T) {
			f := newRecordFixture(t)
			d := rootDraftEditorCreate(t, f, nil)
			p, view := f.principal, f.view
			if mode == "menu" {
				rootDraftEditorRevoke(t, f, "menu.enter")
			}
			if mode == "actor" {
				p.UserID = f.other
			}
			if mode == "view" {
				view = f.table
			}
			got, err := f.service.GetDraftEditorContext(f.ctx, p, f.app, view, d.ID)
			if err == nil || got.Draft.ID != "" || got.EditableFieldIDs != nil {
				t.Fatalf("inaccessible projection %+v %v", got, err)
			}
		})
	}
}
func TestRootDraftEditorReadsDoNotCreateOperationsOrMutateDraft(t *testing.T) {
	f := newRecordFixture(t)
	d := rootDraftEditorCreate(t, f, &f.ownRecord)
	var before, after int
	if err := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE app_id=$1", f.app).Scan(&before); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		got, err := f.service.GetDraftEditorContext(f.ctx, f.principal, f.app, f.view, d.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Draft.DraftVersion != d.DraftVersion {
			t.Fatal("read mutated draft")
		}
	}
	if err := f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.operations WHERE app_id=$1", f.app).Scan(&after); err != nil || before != after {
		t.Fatalf("read wrote operation %d -> %d %v", before, after, err)
	}
}
