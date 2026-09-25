package persistence_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/persistence"
	"github.com/jackc/pgx/v5/pgxpool"
)

func registrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("WEAVEOS_TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("WEAVEOS_TEST_DATABASE_URL must target an isolated PostgreSQL 18 database")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	_, err = pool.Exec(context.Background(), "TRUNCATE auth.authentication_events, auth.invitations, auth.password_credentials, auth.users CASCADE")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "TRUNCATE auth.authentication_events, auth.invitations, auth.password_credentials, auth.users CASCADE")
	})
	return pool
}

func invitation(t *testing.T, pool *pgxpool.Pool, digest []byte) {
	t.Helper()
	ctx := context.Background()
	var adminID string
	if err := pool.QueryRow(ctx, "INSERT INTO auth.users (account, is_bootstrap_admin) VALUES ('Bootstrap', true) ON CONFLICT (account_key) DO UPDATE SET account = EXCLUDED.account RETURNING id").Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO auth.invitations (code_hash, created_by) VALUES ($1, $2)", digest, adminID); err != nil {
		t.Fatal(err)
	}
}

func testRegistrationInput(account string, digest []byte, requestID string) persistence.RegistrationInput {
	return persistence.RegistrationInput{Account: account, PasswordHash: "synthetic-hash-not-a-real-credential", InvitationDigest: digest, ClientIP: "127.0.0.1", UserAgent: "WeaveOS-storage-test", RequestID: requestID}
}

func TestRegisterConsumesInvitationOnceAndRollsBackConflict(t *testing.T) {
	pool := registrationPool(t)
	ctx := context.Background()
	firstCode := make([]byte, 32)
	firstCode[0] = 1
	secondCode := make([]byte, 32)
	secondCode[0] = 2
	invitation(t, pool, firstCode)
	invitation(t, pool, secondCode)
	store := persistence.New(pool)
	user, err := store.Register(ctx, testRegistrationInput(" Alice ", firstCode, "registration-1"))
	if err != nil {
		t.Fatalf("valid invitation and new account must register: %v", err)
	}
	if user.ID == "" || user.Account != "Alice" {
		t.Fatalf("trimmed account and stable ID required, got %+v", user)
	}
	var usedBy string
	if err := pool.QueryRow(ctx, "SELECT used_by FROM auth.invitations WHERE code_hash=$1", firstCode).Scan(&usedBy); err != nil {
		t.Fatal(err)
	}
	if usedBy != user.ID {
		t.Errorf("invitation consumer = %q, want registered user %q", usedBy, user.ID)
	}
	if _, err := store.Register(ctx, testRegistrationInput("Bob", firstCode, "registration-2")); !errors.Is(err, persistence.ErrInvitationUnavailable) {
		t.Errorf("used invitation must be rejected, got %v", err)
	}
	if _, err := store.Register(ctx, testRegistrationInput("Alice", secondCode, "registration-3")); !errors.Is(err, persistence.ErrAccountTaken) {
		t.Errorf("duplicate exact-case account must conflict, got %v", err)
	}
	var unused bool
	if err := pool.QueryRow(ctx, "SELECT used_by IS NULL FROM auth.invitations WHERE code_hash=$1", secondCode).Scan(&unused); err != nil {
		t.Fatal(err)
	}
	if !unused {
		t.Error("account conflict consumed a second invitation")
	}
}

func TestRegisterPreservesCaseAndRejectsInternalSpaces(t *testing.T) {
	pool := registrationPool(t)
	ctx := context.Background()
	firstCode := make([]byte, 32)
	firstCode[0] = 3
	secondCode := make([]byte, 32)
	secondCode[0] = 4
	invitation(t, pool, firstCode)
	invitation(t, pool, secondCode)
	store := persistence.New(pool)
	upper, err := store.Register(ctx, testRegistrationInput("Alice", firstCode, "registration-4"))
	if err != nil {
		t.Fatalf("Alice must register: %v", err)
	}
	lower, err := store.Register(ctx, testRegistrationInput("alice", secondCode, "registration-5"))
	if err != nil {
		t.Fatalf("alice is a distinct account: %v", err)
	}
	if upper.ID == lower.ID || upper.Account == lower.Account {
		t.Errorf("case-distinct accounts collapsed: %+v %+v", upper, lower)
	}
	if _, err := store.Register(ctx, testRegistrationInput("Ali ce", secondCode, "registration-6")); !errors.Is(err, persistence.ErrInvalidAccount) {
		t.Errorf("internal spaces must be rejected, got %v", err)
	}
}
