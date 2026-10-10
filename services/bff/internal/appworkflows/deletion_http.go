package appworkflows

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	"github.com/jackc/pgx/v5"
)

type deletionResult struct {
	OperationID     string     `json:"operationId"`
	FlowID          string     `json:"flowId"`
	Status          string     `json:"status"`
	Reason          *string    `json:"reason"`
	DeletedVersions *int64     `json:"deletedVersions"`
	DeletedAt       *time.Time `json:"deletedAt"`
	CompletedAt     *time.Time `json:"completedAt"`
}

func deletionDTO(d workflowcatalog.Deletion) deletionResult {
	utc := func(t *time.Time) *time.Time {
		if t == nil {
			return nil
		}
		v := t.UTC()
		return &v
	}
	return deletionResult{d.OperationID, d.FlowID, d.Status, d.Reason, d.DeletedVersions, utc(d.DeletedAt), utc(d.CompletedAt)}
}
func readDeletion(ctx context.Context, tx pgx.Tx, actor, app, view, flow, op string) (workflowcatalog.Deletion, error) {
	var d workflowcatalog.Deletion
	e := tx.QueryRow(ctx, `SELECT operation_id::text,flow_id::text,status,reason,engine_deleted_versions,engine_deleted_at,completed_at FROM applications.workflow_deletions WHERE actor_user_id=$1 AND app_id=$2 AND view_id=$3 AND flow_id=$4 AND operation_id=$5`, actor, app, view, flow, op).Scan(&d.OperationID, &d.FlowID, &d.Status, &d.Reason, &d.DeletedVersions, &d.DeletedAt, &d.CompletedAt)
	if errors.Is(e, pgx.ErrNoRows) {
		e = applications.ErrMissing
	}
	return d, e
}
func (a *Application) requestDeletion(ctx context.Context, p session.Principal, app, view, flow string, in lifecycleRequest) (workflowcatalog.Deletion, error) {
	var d workflowcatalog.Deletion
	manager, e := (&applications.Application{Pool: a.Pool}).BeginManagerWrite(ctx, p, app, a.managerOptions()...)
	if e != nil {
		return d, e
	}
	defer manager.Rollback(context.Background())
	raw, _ := json.Marshal(in)
	fingerprint := sha256.Sum256(append([]byte(app+"\x00"+view+"\x00"+flow+"\x00delete\x00"), raw...))
	old, e := manager.Replay(ctx, in.OperationID, "workflow.delete", fingerprint)
	if e != nil {
		return d, e
	}
	if old != nil {
		d, e = readDeletion(ctx, manager.Tx(), p.UserID, app, view, flow, in.OperationID)
	} else {
		if !a.deletionAvailable() {
			return d, session.ErrUnavailable
		}
		if a.RuntimeReady != nil {
			if e = a.RuntimeReady(ctx); e != nil {
				return d, session.ErrUnavailable
			}
		}
		if e = manager.Claim(ctx, in.OperationID, "workflow.delete", fingerprint); e != nil {
			return d, e
		}
		d, e = (workflowcatalog.Catalog{}).RequestDeletionInTx(ctx, manager.Tx(), workflowcatalog.DeletionInput{AppID: app, ViewID: view, FlowID: flow, ActorID: p.UserID, OperationID: in.OperationID, ExpectedRevision: in.ExpectedRevision})
		if e == nil {
			raw, _ = json.Marshal(deletionDTO(d))
			e = manager.Complete(ctx, in.OperationID, applications.Result{Status: 202, Location: "/api/v1/application-operations/" + in.OperationID, Data: raw})
		}
	}
	if e != nil {
		return d, e
	}
	return d, manager.Commit(ctx)
}
func (s *Service) deleteWorkflow(w http.ResponseWriter, r *http.Request, p session.Principal, app, view, flow string) {
	in, e := decodeLifecycle(w, r)
	if e != nil {
		if errors.Is(e, errUnsupportedMediaType) {
			writeEnvelope(w, r, 415, "COMMON_UNSUPPORTED_MEDIA_TYPE", nil)
		} else {
			fail(w, r, e, "")
		}
		return
	}
	d, e := s.Application.requestDeletion(r.Context(), p, app, view, flow, in)
	if e != nil {
		fail(w, r, e, in.OperationID)
		return
	}
	status := 202
	if d.Status == "deleted" {
		status = 200
	}
	raw, _ := json.Marshal(deletionDTO(d))
	w.Header().Set("Location", "/api/v1/applications/"+app+"/forms/"+view+"/workflows/"+flow+"/deletions/"+in.OperationID)
	s.finish(w, r, p, applications.Result{Status: status, Data: raw}, true)
}
func (s *Service) deletionStatus(w http.ResponseWriter, r *http.Request, p session.Principal, app, view, flow, op string) {
	if !validID(op) {
		fail(w, r, errInvalid, "")
		return
	}
	ctx := r.Context()
	manager, e := (&applications.Application{Pool: s.Application.Pool}).BeginManagerWrite(ctx, p, app, s.Application.managerOptions()...)
	if e != nil {
		fail(w, r, e, op)
		return
	}
	defer manager.Rollback(context.Background())
	d, e := readDeletion(ctx, manager.Tx(), p.UserID, app, view, flow, op)
	if e == nil {
		e = manager.Commit(ctx)
	}
	if e != nil {
		fail(w, r, e, op)
		return
	}
	raw, _ := json.Marshal(deletionDTO(d))
	s.finish(w, r, p, applications.Result{Status: 200, Data: raw}, false)
}
