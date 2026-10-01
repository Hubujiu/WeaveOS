package personnel

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Discard the actual business COMMIT acknowledgement, after accepting the
// short read-only prevalidation commit. No fake transaction or mocked outcome.
type queryLostCommit struct {
	net.Conn
	pending *bytes.Reader
	commits *atomic.Int32
	dropped *atomic.Bool
}

func (c *queryLostCommit) Read(p []byte) (int, error) {
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
	if header[0] == 'C' && string(body) == "COMMIT\x00" && c.commits.Add(1) == 2 {
		c.dropped.Store(true)
		_ = c.Conn.Close()
		return 0, io.EOF
	}
	c.pending = bytes.NewReader(append(header, body...))
	return c.pending.Read(p)
}
func TestQ36BusinessCommitFaultNeverReplaysOrPartiallyCleansDraft(t *testing.T) {
	for _, mode := range []string{"lost-ack", "deferred-failure"} {
		t.Run(mode, func(t *testing.T) {
			f, _ := queryWeb(t)
			ctx := context.Background()
			target := newTarget(t, f.fixture)
			qv := baselineVersion(t, f, "")
			zero := int64(0)
			d, err := f.app.CreateDraft(ctx, f.actor, DraftCreateInput{Kind: DraftMemberIdentities, TargetID: &target, BaseVersion: &zero, Payload: []byte(`{"identityIds":[]}`)})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _, _ = f.owner.Exec(ctx, "DELETE FROM personnel.drafts WHERE id=$1", d.ID) })
			var dropped atomic.Bool
			var commits atomic.Int32
			if mode == "lost-ack" {
				cfg := f.app.Pool.Config()
				if cfg.ConnConfig.TLSConfig != nil {
					t.Fatal("isolated plaintext fault transport required")
				}
				cfg.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
					c, e := (&net.Dialer{}).DialContext(ctx, network, address)
					if e != nil {
						return nil, e
					}
					return &queryLostCommit{Conn: c, commits: &commits, dropped: &dropped}, nil
				}
				pool, e := pgxpool.NewWithConfig(ctx, cfg)
				if e != nil {
					t.Fatal(e)
				}
				t.Cleanup(pool.Close)
				f.app.Pool = pool
			} else {
				// A deferred database failure occurs after CleanupDraft, at COMMIT.
				_, err = f.owner.Exec(ctx, `CREATE FUNCTION personnel.q36_reject_draft_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic deferred failure'; END $$;
      CREATE CONSTRAINT TRIGGER q36_reject_draft_commit AFTER DELETE ON personnel.drafts DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION personnel.q36_reject_draft_commit()`)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					_, e := f.owner.Exec(ctx, `DROP TRIGGER q36_reject_draft_commit ON personnel.drafts; DROP FUNCTION personnel.q36_reject_draft_commit()`)
					if e != nil {
						t.Error(e)
					}
				})
			}
			w := f.request("PUT", "/api/v1/personnel/members/"+target+"/identities", queryBody(t, map[string]any{"identityIds": []string{f.i2}, "version": 0, "queryVersion": qv, "draftRef": DraftReference{d.ID, 1}}), true, true)
			queryError(t, w, 503, "COMMON_SERVICE_UNAVAILABLE")
			var version, audits, drafts int
			if err := f.owner.QueryRow(ctx, `SELECT COALESCE((SELECT version FROM personnel.member_configuration WHERE user_id=$1),0),(SELECT count(*) FROM auth.authentication_events WHERE object_id=$1),(SELECT count(*) FROM personnel.drafts WHERE id=$2)`, target, d.ID).Scan(&version, &audits, &drafts); err != nil {
				t.Fatal(err)
			}
			if mode == "lost-ack" {
				if !dropped.Load() || commits.Load() != 2 || version != 1 || audits != 1 || drafts != 0 {
					t.Fatalf("committed once, never replayed: commits=%d version=%d audits=%d drafts=%d dropped=%v", commits.Load(), version, audits, drafts, dropped.Load())
				}
			} else if version != 0 || audits != 0 || drafts != 1 {
				t.Fatal("failed COMMIT must roll back business, audit and draft deletion together")
			}
		})
	}
}
