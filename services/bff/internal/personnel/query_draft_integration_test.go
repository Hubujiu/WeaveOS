package personnel

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestQ36DraftSharedRouteAndIndependentRawLimit(t *testing.T) {
	f := setupWeb(t)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = f.owner.Exec(ctx, "DELETE FROM personnel.drafts WHERE owner_user_id=$1", f.actor.UserID)
	})
	if w := f.request("POST", "/api/v1/personnel/drafts", draftCreateBody, true, false); w.Code != 403 {
		t.Fatal("shared CSRF is required")
	}
	created := f.request("POST", "/api/v1/personnel/drafts", draftCreateBody, true, true)
	if created.Code != 201 {
		t.Fatalf("real shared route must create draft: %d %s", created.Code, created.Body)
	}
	// Canonical payload is allowed; excessive Unicode escape spelling is not.
	payload := map[string]any{"name": "", "description": "", "templateIds": []string{}, "permissionCodes": []string{}}
	codes := []string{}
	// Keep each code under 160 characters and distinct.
	for i := 0; i < 400; i++ {
		codes = append(codes, strings.Repeat("<", 140)+string(rune(0x400+i)))
	}
	payload["permissionCodes"] = codes
	canonical, err := canonicalDraftPayload(DraftIdentity, mustRawJSON(t, payload))
	if err != nil || len(canonical) > 65536 {
		t.Fatalf("independent legal payload: %d %v", len(canonical), err)
	}
	normal := `{"kind":"identity","targetId":null,"baseVersion":null,"payload":` + string(canonical) + `}`
	if len(normal) > 128<<10 {
		t.Fatal("bad normal fixture")
	}
	if w := f.request("POST", "/api/v1/personnel/drafts", normal, true, true); w.Code != 201 {
		t.Fatalf("normal UTF8 wrapper rejected: %d %s", w.Code, w.Body)
	}
	escaped := strings.ReplaceAll(normal, "<", `\u003c`)
	if len(escaped) <= 128<<10 {
		t.Fatal("bad excessive escape fixture")
	}
	var before, after int
	if err = f.owner.QueryRow(ctx, "SELECT count(*) FROM personnel.drafts WHERE owner_user_id=$1", f.actor.UserID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	rejected := f.request("POST", "/api/v1/personnel/drafts", escaped, true, true)
	if rejected.Code != 400 || !strings.Contains(rejected.Body.String(), "COMMON_INVALID_ARGUMENT") {
		t.Fatalf("raw cap must independently reject: %d %s", rejected.Code, rejected.Body)
	}
	if err = f.owner.QueryRow(ctx, "SELECT count(*) FROM personnel.drafts WHERE owner_user_id=$1", f.actor.UserID).Scan(&after); err != nil || after != before {
		t.Fatal("rejected raw encoding changed drafts", err)
	}
}

func mustRawJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
