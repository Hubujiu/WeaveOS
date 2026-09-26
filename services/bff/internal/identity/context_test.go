package identity_test

import (
	"context"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/identity"
)

type externalHeaderKey struct{}

func TestOnlyTrustedInjectionCreatesIdentityContext(t *testing.T) {
	const subjectID = "550e8400-e29b-41d4-a716-446655440001"
	const sessionRef = "550e8400-e29b-41d4-a716-446655440002"
	request := context.WithValue(context.Background(), externalHeaderKey{}, map[string]string{
		"X-User-Id":    subjectID,
		"X-Session-Id": sessionRef,
	})
	if _, ok := identity.From(request); ok {
		t.Fatal("untrusted request headers produced an IdentityContext")
	}
	trusted, err := identity.WithTrusted(request, identity.Context{SubjectID: subjectID, SessionID: sessionRef})
	if err != nil {
		t.Fatalf("validated authentication boundary must inject identity: %v", err)
	}
	actual, ok := identity.From(trusted)
	if !ok || actual.SubjectID != subjectID || actual.SessionID != sessionRef {
		t.Fatalf("business identity = %+v, present=%t; want stable subject/session reference", actual, ok)
	}
	if _, ok := identity.From(request); ok {
		t.Fatal("trusted identity leaked into the original anonymous context")
	}
}

func TestIdentityContextRejectsMissingOrMalformedIdentifiers(t *testing.T) {
	for _, candidate := range []identity.Context{
		{SubjectID: "", SessionID: "550e8400-e29b-41d4-a716-446655440002"},
		{SubjectID: "550e8400-e29b-41d4-a716-446655440001", SessionID: ""},
		{SubjectID: "client-supplied-user", SessionID: "550e8400-e29b-41d4-a716-446655440002"},
		{SubjectID: "550e8400-e29b-41d4-a716-446655440001", SessionID: "replayable-cookie-sid"},
	} {
		if _, err := identity.WithTrusted(context.Background(), candidate); err == nil {
			t.Errorf("malformed identity was injected: %+v", candidate)
		}
	}
}
