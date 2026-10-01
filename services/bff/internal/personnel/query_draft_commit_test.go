package personnel

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestQ36ExactDraftCleanupAllBusinessKindsIncludingNoop(t *testing.T) {
	for _, mode := range []string{"department-new", "department-edit", "department-noop", "identity-edit", "identity-noop", "template-new", "template-edit", "template-noop", "member-groups", "member-groups-noop", "member-identities-noop"} {
		t.Run(mode, func(t *testing.T) {
			f, _ := queryWeb(t)
			ctx := context.Background()
			kind := DraftDepartment
			method, path := "POST", "/api/v1/personnel/departments"
			want := 201
			var target *string
			var base *int64
			body := map[string]any{}
			payload := map[string]any{}
			noop := strings.HasSuffix(mode, "-noop")
			switch {
			case strings.HasPrefix(mode, "department"):
				root := rootDepartment(t, f.fixture)
				body = map[string]any{"name": "draft department", "parentId": root}
				payload = map[string]any{"name": "draft department", "parentId": root}
				if mode != "department-new" {
					d, e := f.app.SaveDepartment(ctx, f.actor, "", DepartmentInput{Name: "draft department", ParentID: root}, RequestMetadata{RequestID: "q36-draft-setup"})
					if e != nil {
						t.Fatal(e)
					}
					cleanDepartment(t, f.fixture, d.ID)
					target = &d.ID
					base = &d.Version
					method = "PUT"
					path += "/" + d.ID
					want = 200
					name := "changed department"
					if noop {
						name = d.Name
					}
					body = map[string]any{"name": name, "version": d.Version}
				}
			case strings.HasPrefix(mode, "identity"), strings.HasPrefix(mode, "template"):
				k := Identity
				kind = DraftIdentity
				path = "/api/v1/personnel/identities"
				id := f.i2
				if strings.HasPrefix(mode, "template") {
					k = Template
					kind = DraftTemplate
					path = "/api/v1/personnel/templates"
					id = f.template
				}
				body = map[string]any{"name": "draft template", "description": "", "permissionCodes": []string{}}
				if mode != "template-new" {
					d, e := f.app.GetDefinition(ctx, f.actor, k, id)
					if e != nil {
						t.Fatal(e)
					}
					target = &id
					base = &d.Version
					method = "PUT"
					path += "/" + id
					want = 200
					body = map[string]any{"name": d.Name, "description": d.Description, "permissionCodes": d.PermissionCodes, "version": d.Version}
					if !noop {
						body["name"] = "changed definition"
					}
					if k == Identity {
						body["templateIds"] = d.TemplateIDs
					}
				}
				payload = map[string]any{"name": body["name"], "description": body["description"], "permissionCodes": body["permissionCodes"]}
				if k == Identity {
					payload["templateIds"] = body["templateIds"]
				}
			default:
				id := newTarget(t, f.fixture)
				zero := int64(0)
				target = &id
				base = &zero
				kind = DraftMemberGroups
				path = "/api/v1/personnel/members/" + id + "/groups"
				want = 200
				op := "add"
				if noop {
					op = "remove"
				}
				root := rootDepartment(t, f.fixture)
				body = map[string]any{"operation": op, "departmentId": root, "version": 0}
				payload = map[string]any{"operation": op, "departmentId": root, "sourceDepartmentId": nil}
				if mode == "member-identities-noop" {
					kind = DraftMemberIdentities
					method = "PUT"
					path = "/api/v1/personnel/members/" + id + "/identities"
					body = map[string]any{"identityIds": []string{}, "version": 0}
					payload = map[string]any{"identityIds": []string{}}
				}
			}
			raw, e := json.Marshal(payload)
			if e != nil {
				t.Fatal(e)
			}
			d, e := f.app.CreateDraft(ctx, f.actor, DraftCreateInput{Kind: kind, TargetID: target, BaseVersion: base, Payload: raw})
			if e != nil {
				t.Fatal(e)
			}
			t.Cleanup(func() { _, _ = f.owner.Exec(ctx, "DELETE FROM personnel.drafts WHERE id=$1", d.ID) })
			body["draftRef"] = DraftReference{d.ID, d.Version}
			if kind == DraftDepartment || kind == DraftMemberGroups || kind == DraftMemberIdentities {
				body["queryVersion"] = baselineVersion(t, f, "")
			}
			before := queryRevisionsForTest(t, f.owner)
			w := f.request(method, path, queryBody(t, body), true, true)
			if w.Code != want {
				t.Fatalf("business kind=%s status=%d body=%s", kind, w.Code, w.Body)
			}
			if target == nil {
				var envelope struct{ Data struct{ ID string } }
				_ = json.Unmarshal(w.Body.Bytes(), &envelope)
				if kind == DraftDepartment {
					cleanDepartment(t, f.fixture, envelope.Data.ID)
				} else {
					cleanupDefinition(t, f.fixture, Template, envelope.Data.ID)
				}
			}
			var exists bool
			if e = f.owner.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM personnel.drafts WHERE id=$1)", d.ID).Scan(&exists); e != nil || exists {
				t.Fatal("exact original target/kind/version draft must be deleted", e)
			}
			if noop && queryRevisionsForTest(t, f.owner) != before {
				t.Fatal("successful no-op may clean draft but must not invalidate business queries")
			}
		})
	}
}
