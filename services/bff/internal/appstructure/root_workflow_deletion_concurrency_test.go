package appstructure

import (
	"context"
	"encoding/json"
	"errors"
	wc "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"testing"
	"time"
)

func TestRootDeletionConfigurationHTTPRejectsAfterFinalization(t *testing.T) {
	for _, action := range []string{"definition", "publish"} {
		t.Run(action, func(t *testing.T) {
			p := rootPublicationSetup(t)
			p.publish(t, uuid(t, p.s.f.owner), 1)
			p.dispatch(t)
			in := deletionInput(t, p)
			acceptDeletion(t, p, in)
			ctx := context.Background()
			var rev int64
			p.s.tx(t, func(tx pgx.Tx) error {
				h, e := (wc.Catalog{}).GetInTx(ctx, tx, in.AppID, in.FlowID)
				if e != nil {
					return e
				}
				h, e = (wc.Catalog{}).FinalizeCloseInTx(ctx, tx, in.AppID, in.FlowID, h.Revision)
				rev = h.Revision
				return e
			})
			body := map[string]any{"operationId": uuid(t, p.s.f.owner), "expectedRevision": rev, "expectedSchemaVersion": 1}
			method := "POST"
			if action == "definition" {
				body = rootWorkflowHTTPBody(t, p.s, p.s.f.actor)
				body["expectedRevision"] = rev
				method = "PUT"
			}
			raw, _ := json.Marshal(body)
			got := rootWorkflowHTTPRequest(t, p.s, p.handler, method, rootWorkflowHTTPPath(p.s, p.flow, action), string(raw), true, true, "", nil)
			if got.Code != 409 {
				t.Fatalf("new %s after accepted deletion must be a domain conflict, got %d: %s", action, got.Code, got.Body.String())
			}
		})
	}
}

func TestRootDeletionConcurrentNewVersionWaitsForAcceptance(t *testing.T) {
	p := rootPublicationSetup(t)
	in := deletionInput(t, p)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	accept, e := p.s.f.runtime.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer accept.Rollback(context.Background())
	writer, e := p.s.f.runtime.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer writer.Rollback(context.Background())
	var first, second int
	if e = accept.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&first); e != nil {
		t.Fatal(e)
	}
	if e = writer.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&second); e != nil {
		t.Fatal(e)
	}
	if _, e = (wc.Catalog{}).RequestDeletionInTx(ctx, accept, in); e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() {
		_, e := writer.Exec(ctx, `INSERT INTO applications.workflow_versions(app_id,flow_id,version,version_id,schema_version,graph_json,bpmn_xml,allow_withdraw,created_by,triggers_json) SELECT app_id,flow_id,version+1,gen_random_uuid(),schema_version,graph_json,bpmn_xml,allow_withdraw,created_by,triggers_json FROM applications.workflow_versions WHERE flow_id=$1 LIMIT 1`, p.flow)
		done <- e
	}()
	for {
		var blocked bool
		if e = p.s.f.owner.QueryRow(ctx, "SELECT $1=ANY(pg_blocking_pids($2))", first, second).Scan(&blocked); e != nil {
			t.Fatal(e)
		}
		if blocked {
			break
		}
		select {
		case e = <-done:
			t.Fatalf("new version did not serialize with accepted deletion: %v", e)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(5 * time.Millisecond):
		}
	}
	if e = accept.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	e = <-done
	var pg *pgconn.PgError
	if !errors.As(e, &pg) || pg.Code != "55000" {
		t.Fatalf("new version ignored committed deletion after lock: %v", e)
	}
}
