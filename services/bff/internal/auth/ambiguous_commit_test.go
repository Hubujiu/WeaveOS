package auth

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"os"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// A real PostgreSQL connection whose transport loses the successful COMMIT
// acknowledgement. The database commits, but pgx must see an uncertain outcome.
type lostCommitAck struct {
	net.Conn
	pending *bytes.Reader
	dropped *atomic.Bool
}

func (c *lostCommitAck) Read(p []byte) (int, error) {
	if c.pending != nil && c.pending.Len() > 0 {
		return c.pending.Read(p)
	}
	header := make([]byte, 5)
	if _, err := io.ReadFull(c.Conn, header); err != nil {
		return 0, err
	}
	size := int(binary.BigEndian.Uint32(header[1:])) - 4
	if size < 0 || size > 16*1024*1024 {
		return 0, io.ErrUnexpectedEOF
	}
	body := make([]byte, size)
	if _, err := io.ReadFull(c.Conn, body); err != nil {
		return 0, err
	}
	if header[0] == 'C' && string(body) == "COMMIT\x00" {
		c.dropped.Store(true)
		_ = c.Conn.Close()
		return 0, io.EOF
	}
	c.pending = bytes.NewReader(append(header, body...))
	return c.pending.Read(p)
}
func TestUncertainCommitNeverClaimsSuccess(t *testing.T) {
	for _, operation := range []string{"invitation", "reset"} {
		t.Run(operation, func(t *testing.T) {
			a := setup(t)
			a.user(t, "admin", true)
			target := a.user(t, "member", false)
			cookies := a.login(t, "admin")
			cfg, err := pgxpool.ParseConfig(os.Getenv("WEAVEOS_TEST_DATABASE_URL"))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.ConnConfig.TLSConfig != nil {
				t.Fatal("isolated fault transport requires plaintext PostgreSQL")
			}
			var dropped atomic.Bool
			cfg.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
				conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
				if err != nil {
					return nil, err
				}
				return &lostCommitAck{Conn: conn, dropped: &dropped}, nil
			}
			pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			a.service.Pool = pool
			path := "/api/v1/invitations"
			if operation == "reset" {
				path = "/api/v1/users/" + target + "/password-reset"
			}
			response := a.request("POST", path, map[string]string{}, cookies, nil)
			if !dropped.Load() {
				t.Fatal("real successful COMMIT acknowledgement was not intercepted")
			}
			checkStatus(t, response, 503)
			var count int
			if operation == "invitation" {
				err = a.pool.QueryRow(context.Background(), "SELECT count(*) FROM auth.invitations").Scan(&count)
			} else {
				err = a.pool.QueryRow(context.Background(), "SELECT auth_version-1 FROM auth.users WHERE id=$1", target).Scan(&count)
			}
			if err != nil || count != 1 {
				t.Fatal("fault must occur after actual commit, with no automatic replay")
			}
			if bytes.Contains(response.Body.Bytes(), []byte("invitationCode")) {
				t.Fatal("uncertain commit must not expose a successful invitation result")
			}
			if len(response.Result().Cookies()) != 0 {
				t.Fatal("uncertain commit must not renew or invent cookies")
			}
			checkStatus(t, a.request("GET", "/api/v1/sessions/current", nil, cookies, nil), 200)
		})
	}
}
