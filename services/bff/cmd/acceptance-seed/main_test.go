package main

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAcceptanceSeedProducesIsolatedUsableFixtures(t *testing.T) {
	url := os.Getenv("WEAVEOS_TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("WEAVEOS_TEST_DATABASE_URL must target an isolated PostgreSQL 18 database")
	}
	output := filepath.Join(t.TempDir(), "acceptance-fixtures.json")
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	var databaseName string
	if err := pool.QueryRow(ctx, "SELECT current_database()").Scan(&databaseName); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(databaseName, "weaveos_") {
		t.Fatalf("refusing fixture cleanup outside a weaveos_ test database: %q", databaseName)
	}
	clearFixtures := func() {
		if _, err := pool.Exec(ctx, "TRUNCATE auth.authentication_events, auth.invitations, auth.password_credentials, auth.users CASCADE"); err != nil {
			t.Fatal(err)
		}
	}
	clearFixtures()
	t.Cleanup(func() { clearFixtures(); pool.Close() })
	if err := run(ctx, config{databaseURL: url, outputPath: output}); err != nil {
		t.Fatalf("isolated acceptance seed must run: %v", err)
	}
	info, err := os.Stat(output)
	if err != nil {
		t.Fatalf("fixture file not written: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("fixture file is empty")
	}
	if runtime.GOOS == "windows" {
		current, err := user.Current()
		if err != nil {
			t.Fatal(err)
		}
		acl, err := exec.Command("icacls", output).Output()
		if err != nil {
			t.Fatal(err)
		}
		entries := strings.Split(strings.TrimSpace(string(acl)), "\n")
		if len(entries) != 3 || !strings.Contains(strings.ToLower(entries[0]), strings.ToLower(current.Username+":(F)")) || strings.Contains(string(acl), ":(I)") || strings.Count(string(acl), ":(F)") != 1 {
			t.Errorf("fixture ACL grants unexpected access: %s", acl)
		}
	} else if info.Mode().Perm()&0o077 != 0 {
		t.Errorf("fixture file is readable by other users: %v", info.Mode().Perm())
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Admin         struct{ Account, Password string }
		User          struct{ Account, Password string }
		Disabled      struct{ Account, Password string }
		ResetTarget   struct{ ID, Account string }
		Invitations   map[string]string
		UIInvitations map[string]string `json:"uiInvitations"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Admin.Account == "" || fixture.Admin.Password == "" || fixture.User.Account == "" || fixture.User.Password == "" || fixture.Disabled.Account == "" || fixture.ResetTarget.ID == "" {
		t.Fatalf("fixture lacks required independent users: %+v", fixture)
	}
	for _, name := range []string{"valid", "concurrent", "rollback", "passwordPolicy"} {
		if fixture.Invitations[name] == "" {
			t.Errorf("missing %s invitation", name)
		}
	}
	for _, name := range []string{"chromium", "firefox", "webkit"} {
		if fixture.UIInvitations[name] == "" {
			t.Errorf("missing %s UI invitation", name)
		}
	}
	var hash string
	if err := pool.QueryRow(ctx, "SELECT c.password_hash FROM auth.users u JOIN auth.password_credentials c ON c.user_id=u.id WHERE u.account_key=$1", fixture.User.Account).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if hash == fixture.User.Password || !strings.HasPrefix(hash, "$argon2id$") {
		t.Error("seed stored a plaintext or non-Argon2id password")
	}
	var disabledStatus string
	if err := pool.QueryRow(ctx, "SELECT status FROM auth.users WHERE account_key=$1", fixture.Disabled.Account).Scan(&disabledStatus); err != nil {
		t.Fatal(err)
	}
	if disabledStatus != "disabled" {
		t.Errorf("disabled fixture status = %q", disabledStatus)
	}
}

func TestAcceptanceSeedRejectsNonTestDatabaseBeforeMutation(t *testing.T) {
	testURL := os.Getenv("WEAVEOS_TEST_DATABASE_URL")
	if testURL == "" {
		t.Fatal("WEAVEOS_TEST_DATABASE_URL must target an isolated PostgreSQL 18 database")
	}
	parsed, err := url.Parse(testURL)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Path = "/postgres"
	unsafeURL := parsed.String()
	output := filepath.Join(t.TempDir(), "must-not-exist.json")
	err = run(context.Background(), config{databaseURL: unsafeURL, outputPath: output})
	if err == nil || !strings.Contains(err.Error(), "refusing non-test database") {
		t.Fatalf("seed must reject non-test database before writes, got %v", err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Errorf("unsafe seed left a fixture file: %v", err)
	}
}
