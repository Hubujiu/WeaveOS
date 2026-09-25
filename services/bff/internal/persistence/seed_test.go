package persistence_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/persistence"
)

func TestBootstrapSeedIsIdempotentAndDoesNotChangeExistingAccount(t *testing.T) {
	pool := registrationPool(t)
	ctx := context.Background()
	store := persistence.New(pool)
	first, err := store.SeedBootstrap(ctx, persistence.BootstrapSeedInput{Account: "Bootstrap", PasswordHash: "synthetic-hash-one", RequestID: "seed-1"})
	if err != nil {
		t.Fatalf("first controlled seed must create admin: %v", err)
	}
	if first.ID == "" || first.Account != "Bootstrap" {
		t.Fatalf("seed result invalid: %+v", first)
	}
	if _, err := pool.Exec(ctx, "UPDATE auth.users SET status='disabled', auth_version=auth_version+1 WHERE id=$1", first.ID); err != nil {
		t.Fatal(err)
	}
	second, err := store.SeedBootstrap(ctx, persistence.BootstrapSeedInput{Account: "Bootstrap", PasswordHash: "synthetic-hash-two", RequestID: "seed-2"})
	if err != nil {
		t.Fatalf("repeated seed must be idempotent: %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("seed created another admin: first=%s second=%s", first.ID, second.ID)
	}
	var hash, status string
	var version int64
	if err := pool.QueryRow(ctx, "SELECT c.password_hash, u.status, u.auth_version FROM auth.users u JOIN auth.password_credentials c ON c.user_id=u.id WHERE u.id=$1", first.ID).Scan(&hash, &status, &version); err != nil {
		t.Fatal(err)
	}
	if hash != "synthetic-hash-one" || status != "disabled" || version != 2 {
		t.Errorf("repeated seed changed password/status/version: hash=%q status=%q version=%d", hash, status, version)
	}
}

func TestBootstrapSeedRefusesToPromoteSameNameOrdinaryAccount(t *testing.T) {
	pool := registrationPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "INSERT INTO auth.users(account) VALUES ('Bootstrap')"); err != nil {
		t.Fatal(err)
	}
	store := persistence.New(pool)
	if _, err := store.SeedBootstrap(ctx, persistence.BootstrapSeedInput{Account: "Bootstrap", PasswordHash: "synthetic-hash", RequestID: "seed-3"}); !errors.Is(err, persistence.ErrBootstrapCollision) {
		t.Fatalf("ordinary account must not be promoted, got %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM auth.users WHERE is_bootstrap_admin").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("ordinary account was promoted: admin count=%d", count)
	}
}
