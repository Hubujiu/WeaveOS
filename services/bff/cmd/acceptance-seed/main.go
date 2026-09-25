// Command acceptance-seed creates synthetic fixtures in an isolated migrated database.
// The output contains passwords and invitation codes and must never be committed.
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/persistence"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/argon2"
)

type config struct {
	databaseURL string
	outputPath  string
}

type credentials struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

type fixtureUser struct {
	ID      string `json:"id"`
	Account string `json:"account"`
}

type fixture struct {
	Admin         credentials       `json:"admin"`
	User          credentials       `json:"user"`
	Disabled      credentials       `json:"disabled"`
	ResetTarget   fixtureUser       `json:"resetTarget"`
	Invitations   map[string]string `json:"invitations"`
	UIInvitations map[string]string `json:"uiInvitations"`
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func passwordHash(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, 2, 19*1024, 1, 32)
	return fmt.Sprintf("$argon2id$v=19$m=19456,t=2,p=1$%s$%s",
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func restrictFile(path string) error {
	if runtime.GOOS != "windows" {
		return nil // O_CREATE mode 0600 is effective on Unix.
	}
	current, err := user.Current()
	if err != nil {
		return err
	}
	// Remove inherited access before writing any synthetic credentials.
	output, err := exec.Command("icacls", path, "/inheritance:r", "/grant:r", "*"+current.Uid+":F").CombinedOutput()
	if err != nil {
		return fmt.Errorf("restrict fixture ACL: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func run(ctx context.Context, cfg config) (runErr error) {
	if cfg.databaseURL == "" || cfg.outputPath == "" {
		return errors.New("isolated database URL and private output path are required")
	}
	parsed, err := pgxpool.ParseConfig(cfg.databaseURL)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(parsed.ConnConfig.Database, "weaveos_") ||
		(parsed.ConnConfig.Host != "127.0.0.1" && parsed.ConnConfig.Host != "localhost" && parsed.ConnConfig.Host != "::1") {
		return errors.New("refusing non-test database: use a local weaveos_ database")
	}
	file, err := os.OpenFile(cfg.outputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
		if runErr != nil {
			_ = os.Remove(cfg.outputPath)
		}
	}()
	if err := restrictFile(cfg.outputPath); err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, cfg.databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return err
	}
	store := persistence.New(pool)
	admin := credentials{}
	admin.Account, err = randomToken(12)
	if err != nil {
		return err
	}
	admin.Account = "acceptance-admin-" + admin.Account
	secret, err := randomToken(24)
	if err != nil {
		return err
	}
	admin.Password = "Aa1!" + secret
	adminHash, err := passwordHash(admin.Password)
	if err != nil {
		return err
	}
	adminUser, err := store.SeedBootstrap(ctx, persistence.BootstrapSeedInput{Account: admin.Account, PasswordHash: adminHash, RequestID: "acceptance-seed-admin"})
	if err != nil {
		return err
	}
	result := fixture{Admin: admin, Invitations: make(map[string]string), UIInvitations: make(map[string]string)}
	createInvite := func() (string, error) {
		code, err := randomToken(32)
		if err != nil {
			return "", err
		}
		digest := sha256.Sum256([]byte(code))
		_, err = pool.Exec(ctx, "INSERT INTO auth.invitations (code_hash, created_by) VALUES ($1, $2)", digest[:], adminUser.ID)
		return code, err
	}
	createUser := func(label string) (credentials, persistence.User, error) {
		var account credentials
		token, err := randomToken(12)
		if err != nil {
			return account, persistence.User{}, err
		}
		account.Account = "acceptance-" + label + "-" + token
		secret, err := randomToken(24)
		if err != nil {
			return account, persistence.User{}, err
		}
		account.Password = "Aa1!" + secret
		hash, err := passwordHash(account.Password)
		if err != nil {
			return account, persistence.User{}, err
		}
		code, err := createInvite()
		if err != nil {
			return account, persistence.User{}, err
		}
		digest := sha256.Sum256([]byte(code))
		user, err := store.Register(ctx, persistence.RegistrationInput{Account: account.Account, PasswordHash: hash, InvitationDigest: digest[:], RequestID: "acceptance-seed-" + label})
		return account, user, err
	}
	result.User, _, err = createUser("user")
	if err != nil {
		return err
	}
	var disabled persistence.User
	result.Disabled, disabled, err = createUser("disabled")
	if err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, "UPDATE auth.users SET status='disabled', auth_version=auth_version+1 WHERE id=$1", disabled.ID); err != nil {
		return err
	}
	var resetTarget persistence.User
	_, resetTarget, err = createUser("reset")
	if err != nil {
		return err
	}
	result.ResetTarget = fixtureUser{ID: resetTarget.ID, Account: resetTarget.Account}
	for _, name := range []string{"valid", "concurrent", "rollback", "passwordPolicy"} {
		result.Invitations[name], err = createInvite()
		if err != nil {
			return err
		}
	}
	for _, browser := range []string{"chromium", "firefox", "webkit"} {
		result.UIInvitations[browser], err = createInvite()
		if err != nil {
			return err
		}
	}
	if err := json.NewEncoder(file).Encode(result); err != nil {
		return err
	}
	return file.Sync()
}

func main() {
	cfg := config{databaseURL: os.Getenv("WEAVEOS_TEST_DATABASE_URL"), outputPath: os.Getenv("WEAVEOS_ACCEPTANCE_FIXTURES")}
	if err := run(context.Background(), cfg); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "acceptance seed failed:", strings.ReplaceAll(err.Error(), cfg.databaseURL, "[database URL redacted]"))
		os.Exit(1)
	}
}
