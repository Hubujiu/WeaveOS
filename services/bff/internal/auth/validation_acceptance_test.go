package auth

import (
	"encoding/json"
	"testing"
)

// ADR-002's accepted field-validation example supplies the code, location and public names.
func TestFieldValidationUsesApprovedEnvelope(t *testing.T) {
	a := setup(t)
	for _, fixture := range []struct {
		path, field, code string
		body              map[string]string
	}{
		{"/api/v1/registrations", "account", "VALIDATION_REQUIRED", map[string]string{"password": "Aa1!", "invitationCode": "synthetic"}},
		{"/api/v1/registrations", "password", "AUTH_PASSWORD_POLICY_VIOLATION", map[string]string{"account": "member", "password": "Aa1!中", "invitationCode": "synthetic"}},
		{"/api/v1/registrations", "invitationCode", "VALIDATION_REQUIRED", map[string]string{"account": "member", "password": "Aa1!"}},
		{"/api/v1/sessions", "password", "VALIDATION_REQUIRED", map[string]string{"account": "member", "password": ""}},
	} {
		r := a.request("POST", fixture.path, fixture.body, nil, nil)
		if r.Code != 400 {
			t.Fatalf("field validation status=%d want400", r.Code)
		}
		var body struct {
			Code string
			Data struct {
				Violations []struct{ Location, Field, Code, Message string }
			}
		}
		if err := json.Unmarshal(r.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Code != "COMMON_VALIDATION_FAILED" {
			t.Errorf("field violation envelope code=%s, want COMMON_VALIDATION_FAILED", body.Code)
			continue
		}
		found := false
		for _, v := range body.Data.Violations {
			if v.Location == "body" && v.Field == fixture.field && v.Code == fixture.code && v.Message != "" {
				found = true
			}
		}
		if !found {
			t.Error("approved public field violation missing")
		}
	}
}
