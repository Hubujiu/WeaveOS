package auth

import "testing"

// Q13 (2026-09-26), PRD FR-018: printable ASCII only, no extra length floor.
func TestPasswordPrintableASCII(t *testing.T) {
	for _, password := range []string{"Aa1!", " Aa1! ", "Aa1~", "Aa1[", "Aa1`"} {
		if !validPassword(password) { t.Errorf("printable ASCII fixture rejected") }
	}
	for _, password := range []string{"Aa1 ", "Aa1中", "Aa1!中", "Aa1!é", "Aa1!\u00a0", "Aa1!\t", "Aa1!\x00", "Aa1!\x7f"} {
		if validPassword(password) { t.Errorf("nonconforming fixture accepted: %q", password) }
	}
}

func TestRegistrationRejectsNonPrintableASCIIBeforeConsumingInvitation(t *testing.T) {
	a := setup(t)
	admin := a.user(t, "bootstrap", true)
	code := a.invitation(t, admin, 91)
	for _, password := range []string{"Aa1!中", "Aa1!\t", "Aa1 "} {
		r := a.request("POST", "/api/v1/registrations", map[string]string{"account":"ascii-member", "password":password, "invitationCode":code}, nil, nil)
		if r.Code != 400 { t.Fatalf("Q13 registration status=%d, want 400", r.Code) }
	}
	r := a.request("POST", "/api/v1/registrations", map[string]string{"account":"ascii-member", "password":" Aa1! ", "invitationCode":code}, nil, nil)
	if r.Code != 201 { t.Fatalf("valid printable ASCII registration status=%d, want 201", r.Code) }
	login := a.request("POST", "/api/v1/sessions", map[string]string{"account":"ascii-member", "password":" Aa1! "}, nil, nil)
	if login.Code != 201 { t.Fatal("password spaces must be preserved") }
	trimmed := a.request("POST", "/api/v1/sessions", map[string]string{"account":"ascii-member", "password":"Aa1!"}, nil, nil)
	if trimmed.Code != 401 { t.Fatal("password must not be trimmed") }
}
