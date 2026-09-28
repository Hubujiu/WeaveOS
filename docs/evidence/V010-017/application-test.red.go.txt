package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

// ADR-005 D1 / PRD FR-01: these are real use-case tests, without HTTP requests,
// response writers, cookies or mock storage. Product expectations are inherited
// from ADR-001 and the registration/session specification, not current outputs.
func applicationFor(a *testApp) *Application {
	return &Application{Pool: a.pool, Sessions: a.service.Sessions, AuditKeyID: a.service.AuditKeyID, AuditKey: a.service.AuditKey, Logger: a.service.Logger}
}

func applicationMetadata() RequestMetadata {
	return RequestMetadata{ClientIP: "192.0.2.30", UserAgent: "application-test", RequestID: "application-boundary-test"}
}

func requireFailure(t *testing.T, err error, code string) {
	t.Helper()
	var failure *Failure
	if !errors.As(err, &failure) || failure.Code != code {
		t.Fatalf("expected application failure %s (details omitted)", code)
	}
}

func TestApplicationRegistrationWithoutHTTP(t *testing.T) {
	a := setup(t)
	app := applicationFor(a)
	ctx := context.Background()
	admin := a.user(t, "admin", true)
	code := a.invitation(t, admin, 21)
	user, err := app.Register(ctx, "  direct-member  ", "Aa1!", code, applicationMetadata())
	if err != nil || user.ID == "" || user.Account != "direct-member" {
		t.Fatal("registration must work without HTTP and normalize account")
	}
	if n, err := a.client.DBSize(ctx).Result(); err != nil || n != 0 {
		t.Fatal("registration must not create a session")
	}
	var requestID string
	if err := a.pool.QueryRow(ctx, "SELECT request_id FROM auth.authentication_events WHERE subject_user_id=$1 AND event_type='register' AND outcome='success'", user.ID).Scan(&requestID); err != nil || requestID != applicationMetadata().RequestID {
		t.Fatal("registration must atomically retain supplied audit metadata")
	}
	_, err = app.Register(ctx, "second-member", "Aa1!", code, applicationMetadata())
	requireFailure(t, err, "INVITATION_ALREADY_USED")
	unused := a.invitation(t, admin, 22)
	_, err = app.Register(ctx, "direct-member", "Aa1!", unused, applicationMetadata())
	requireFailure(t, err, "USER_ACCOUNT_ALREADY_EXISTS")
	if _, err := app.Register(ctx, "after-conflict", "Aa1!", unused, applicationMetadata()); err != nil {
		t.Fatal("failed registration must not consume invitation")
	}
}

func TestApplicationLoginWithoutHTTP(t *testing.T) {
	a := setup(t)
	app := applicationFor(a)
	id := a.user(t, "member", false)
	ctx := context.Background()
	result, err := app.Login(ctx, "member", "Aa1!", applicationMetadata())
	if err != nil || result.User.ID != id || result.User.Account != "member" || result.SID == "" || result.CSRF == "" {
		t.Fatal("login must return authenticated identity and session material without HTTP")
	}
	record, err := a.service.Sessions.Load(ctx, result.SID)
	if err != nil || record.UserID != id {
		t.Fatal("login must create a real server-side session")
	}
	_, err = app.Login(ctx, "member", "Wrong1!", applicationMetadata())
	requireFailure(t, err, "AUTH_INVALID_CREDENTIALS")
	_, err = app.Login(ctx, "unknown-member", "Aa1!", applicationMetadata())
	requireFailure(t, err, "AUTH_INVALID_CREDENTIALS")
}

func applicationAdmin(id string) session.Principal {
	return session.Principal{UserID: id, BootstrapAdmin: true, SessionRef: "11111111-1111-4111-8111-111111111111", Record: session.Record{AuthVersion: "1"}}
}

func TestApplicationInvitationWithoutHTTP(t *testing.T) {
	a := setup(t)
	app := applicationFor(a)
	ctx := context.Background()
	admin := a.user(t, "admin", true)
	result, err := app.CreateInvitation(ctx, applicationAdmin(admin), applicationMetadata())
	if err != nil || result.ID == "" || result.Code == "" {
		t.Fatal("administrator invitation use case must work without HTTP")
	}
	if _, err := app.Register(ctx, "invited-member", "Aa1!", result.Code, applicationMetadata()); err != nil {
		t.Fatal("returned invitation must allow exactly one registration")
	}
	member := a.user(t, "ordinary-member", false)
	// Even a caller-supplied admin flag cannot replace authoritative DB state.
	_, err = app.CreateInvitation(ctx, applicationAdmin(member), applicationMetadata())
	requireFailure(t, err, "COMMON_PERMISSION_DENIED")
}

func TestApplicationPasswordResetWithoutHTTP(t *testing.T) {
	a := setup(t)
	app := applicationFor(a)
	ctx := context.Background()
	admin := a.user(t, "admin", true)
	target := a.user(t, "target", false)
	var before int64
	if err := a.pool.QueryRow(ctx, "SELECT auth_version FROM auth.users WHERE id=$1", target).Scan(&before); err != nil {
		t.Fatal("could not inspect isolated fixture")
	}
	if err := app.ResetPassword(ctx, applicationAdmin(admin), target, applicationMetadata()); err != nil {
		t.Fatal("administrator reset use case must work without HTTP")
	}
	var after int64
	if err := a.pool.QueryRow(ctx, "SELECT auth_version FROM auth.users WHERE id=$1", target).Scan(&after); err != nil || after != before+1 {
		t.Fatal("reset must atomically advance the authentication version")
	}
	_, err := app.Login(ctx, "target", "Aa1!", applicationMetadata())
	requireFailure(t, err, "AUTH_INVALID_CREDENTIALS")
	if _, err := app.Login(ctx, "target", "Abc@123456", applicationMetadata()); err != nil {
		t.Fatal("current approved reset password must authenticate")
	}
}
