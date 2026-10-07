package apprecordservice

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/jackc/pgx/v5/pgxpool"
	"testing"
	"time"
)

// A replay must not reserve one connection while waiting for a second from the same pool.
func TestRootWorkflowSaveReplayUsesOnePoolConnection(t *testing.T) {
	f := rootSaveSetup(t, true)
	req := rootSaveRequest(t, f, "saved-once")
	first := rootSave(t, f, req)
	cfg := f.runtime.Config()
	cfg.MaxConns = 1
	cfg.MinConns = 0
	pool, err := pgxpool.NewWithConfig(f.ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	service := *f.service
	service.Pool = pool
	ctx, cancel := context.WithTimeout(f.ctx, 3*time.Second)
	defer cancel()
	again, err := service.SaveWorkflowTask(ctx, f.principal, req, applications.Metadata{RequestID: "root-v043-replay-single-connection"})
	if err != nil {
		t.Fatalf("confirmed replay must complete without a nested pool acquisition: %v", err)
	}
	if first != again {
		t.Fatalf("replay changed result: %+v != %+v", first, again)
	}
	rootSaveRow(t, f, "saved-once", 2)
	rootSaveCounts(t, f, 1)
}

func TestRootWorkflowSaveReplayStillChecksCurrentReadPermission(t *testing.T) {
	f := rootSaveSetup(t, true)
	req := rootSaveRequest(t, f, "saved-once")
	rootSave(t, f, req)
	rootSaveExec(t, f, "UPDATE applications.permission_groups SET enabled=false WHERE app_id=$1", f.app)
	result, err := f.service.SaveWorkflowTask(f.ctx, f.principal, req, applications.Metadata{RequestID: "root-v043-replay-revoked-read"})
	if err == nil || result.ID != "" {
		t.Fatalf("confirmed operation bypassed current read authorization: %+v, %v", result, err)
	}
	rootSaveRow(t, f, "saved-once", 2)
	rootSaveCounts(t, f, 1)
}
