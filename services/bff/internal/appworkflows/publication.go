package appworkflows

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/personnel"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	pb "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc/pb"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type DeploymentClient interface {
	Deploy(context.Context, *pb.DeployRequest) (*pb.DeploymentReceipt, error)
	Lookup(context.Context, *pb.LookupRequest) (*pb.LookupResponse, error)
}

var errPublicationReceipt = errors.New("publication receipt does not match immutable history")

type publicationRequest struct {
	OperationID           string `json:"operationId"`
	ExpectedRevision      int64  `json:"expectedRevision"`
	ExpectedSchemaVersion int64  `json:"expectedSchemaVersion"`
}
type publicationResult struct {
	OperationID string  `json:"operationId"`
	FlowID      string  `json:"flowId"`
	Version     int64   `json:"version"`
	Status      string  `json:"status"`
	Reason      *string `json:"reason"`
}
type publicationIntent struct {
	ActorID, OperationID, AppID, ViewID, FlowID, VersionID, Token string
	Version, AuthVersion, CloseEpoch                              int64
	Hash                                                          []byte
	Attempts                                                      int
	BPMN                                                          string
}

func (a *Application) deploymentAvailable() bool {
	if a == nil || a.DeploymentClient == nil {
		return false
	}
	v := reflect.ValueOf(a.DeploymentClient)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !v.IsNil()
	}
	return true
}
func (a *Application) managerOptions() []applications.WriteOptions {
	if a.Limits.LockTimeout > 0 && a.Limits.StatementTimeout > 0 {
		return []applications.WriteOptions{{LockTimeout: a.Limits.LockTimeout, StatementTimeout: a.Limits.StatementTimeout}}
	}
	return nil
}
func (s *Service) publish(w http.ResponseWriter, r *http.Request, p session.Principal, appID, viewID, flowID string) {
	raw, e := strictJSON(w, r)
	if e != nil {
		if errors.Is(e, errUnsupportedMediaType) {
			writeEnvelope(w, r, 415, "COMMON_UNSUPPORTED_MEDIA_TYPE", nil)
		} else {
			fail(w, r, e, "")
		}
		return
	}
	if _, e = exactObject(raw, []string{"operationId", "expectedRevision", "expectedSchemaVersion"}, nil); e != nil {
		fail(w, r, e, "")
		return
	}
	var q publicationRequest
	if json.Unmarshal(raw, &q) != nil || !validID(q.OperationID) || q.ExpectedRevision < 1 || q.ExpectedRevision > maxSafeInteger || q.ExpectedSchemaVersion < 1 || q.ExpectedSchemaVersion > maxSafeInteger {
		fail(w, r, errInvalid, "")
		return
	}
	ctx := r.Context()
	manager, e := (&applications.Application{Pool: s.Application.Pool}).BeginManagerWrite(ctx, p, appID, s.Application.managerOptions()...)
	if e != nil {
		fail(w, r, e, q.OperationID)
		return
	}
	defer manager.Rollback(context.Background())
	tx := manager.Tx()
	canonical, _ := json.Marshal(q)
	fingerprint := sha256.Sum256(append([]byte(appID+"\x00"+viewID+"\x00"+flowID+"\x00publish\x00"), canonical...))
	var got publicationResult
	var prior []byte
	var oldApp, oldView string
	e = tx.QueryRow(ctx, `SELECT app_id::text,view_id::text,operation_id::text,flow_id::text,version,status,reason,fingerprint FROM applications.workflow_publications WHERE actor_user_id=$1 AND operation_id=$2`, p.UserID, q.OperationID).Scan(&oldApp, &oldView, &got.OperationID, &got.FlowID, &got.Version, &got.Status, &got.Reason, &prior)
	if e == nil {
		if oldApp != appID || oldView != viewID || got.FlowID != flowID || !bytes.Equal(prior, fingerprint[:]) {
			fail(w, r, applications.ErrOperationConflict, q.OperationID)
			return
		}
	} else if !errors.Is(e, pgx.ErrNoRows) {
		fail(w, r, e, q.OperationID)
		return
	} else {
		if !s.Application.deploymentAvailable() {
			fail(w, r, session.ErrUnavailable, q.OperationID)
			return
		}
		v, err := (workflowcatalog.Catalog{}).PublicationVersionInTx(ctx, tx, appID, flowID, 0)
		if err != nil {
			fail(w, r, err, q.OperationID)
			return
		}
		if v.Head.ViewID != viewID {
			fail(w, r, errWorkflowMiss, q.OperationID)
			return
		}
		if v.Head.Revision != q.ExpectedRevision || v.SchemaVersion != q.ExpectedSchemaVersion {
			fail(w, r, workflowcatalog.ErrConflict, q.OperationID)
			return
		}
		if v.Head.State == "closing" {
			fail(w, r, workflowcatalog.ErrClosing, q.OperationID)
			return
		}
		if !v.Compatible {
			fail(w, r, workflowcatalog.ErrNotReady, q.OperationID)
			return
		}
		if err = authorizeAssignees(ctx, tx, appID, viewID, v.Graph); err != nil {
			fail(w, r, err, q.OperationID)
			return
		}
		hash := sha256.Sum256([]byte(v.BPMN))
		got = publicationResult{OperationID: q.OperationID, FlowID: flowID, Version: v.Version, Status: "pending"}
		_, err = tx.Exec(ctx, `INSERT INTO applications.workflow_publications(actor_user_id,operation_id,app_id,view_id,flow_id,version,version_id,bpmn_sha256,actor_auth_version,accepted_close_epoch,expected_revision,expected_schema_version,fingerprint) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, p.UserID, q.OperationID, appID, viewID, flowID, v.Version, v.VersionID, hash[:], p.Record.AuthVersion, v.CloseEpoch, q.ExpectedRevision, q.ExpectedSchemaVersion, fingerprint[:])
		if err != nil {
			var duplicate *pgconn.PgError
			if errors.As(err, &duplicate) && duplicate.Code == "23505" {
				err = applications.ErrOperationConflict
			}
			fail(w, r, err, q.OperationID)
			return
		}
		if err = publicationAudit(ctx, tx, p.UserID, appID, viewID, got, "requested", httpserver.Metadata(ctx).RequestID); err != nil {
			fail(w, r, err, q.OperationID)
			return
		}
	}
	if e = manager.Commit(ctx); e != nil {
		fail(w, r, e, q.OperationID)
		return
	}
	code := http.StatusOK
	if got.Status == "pending" || got.Status == "unknown" {
		code = http.StatusAccepted
		w.Header().Set("Location", r.URL.Path[:len(r.URL.Path)-len("publish")]+"publications/"+q.OperationID)
	}
	b, _ := json.Marshal(got)
	s.finish(w, r, p, applications.Result{Status: code, Data: b}, true)
}
func (s *Service) publicationStatus(w http.ResponseWriter, r *http.Request, p session.Principal, appID, viewID, flowID, id string) {
	if !validID(id) {
		fail(w, r, errInvalid, "")
		return
	}
	ctx := r.Context()
	manager, e := (&applications.Application{Pool: s.Application.Pool}).BeginManagerWrite(ctx, p, appID, s.Application.managerOptions()...)
	if e != nil {
		fail(w, r, e, id)
		return
	}
	defer manager.Rollback(context.Background())
	var got publicationResult
	e = manager.Tx().QueryRow(ctx, `SELECT operation_id::text,flow_id::text,version,status,reason FROM applications.workflow_publications WHERE actor_user_id=$1 AND operation_id=$2 AND app_id=$3 AND view_id=$4 AND flow_id=$5`, p.UserID, id, appID, viewID, flowID).Scan(&got.OperationID, &got.FlowID, &got.Version, &got.Status, &got.Reason)
	if errors.Is(e, pgx.ErrNoRows) {
		e = applications.ErrMissing
	}
	if e != nil {
		fail(w, r, e, id)
		return
	}
	if e = manager.Commit(ctx); e != nil {
		fail(w, r, e, id)
		return
	}
	b, _ := json.Marshal(got)
	s.finish(w, r, p, applications.Result{Status: 200, Data: b}, false)
}
func publicationAudit(ctx context.Context, tx pgx.Tx, actor, appID, viewID string, r publicationResult, action, requestID string) error {
	summary, e := json.Marshal(map[string]any{"appId": appID, "flowId": r.FlowID, "operationId": r.OperationID, "version": r.Version, "status": r.Status, "action": "workflow.publish." + action})
	if e != nil {
		return e
	}
	outcome := "success"
	if action == "blocked" {
		outcome = "failure"
	}
	_, e = tx.Exec(ctx, `INSERT INTO auth.authentication_events(event_type,outcome,actor_user_id,reason_code,request_id,object_type,object_id,change_summary) VALUES('application_structure_changed',$1,$2,$3,$4,'form',$5,$6)`, outcome, actor, "WORKFLOW_PUBLICATION_"+strings.ToUpper(action), requestID, viewID, summary)
	return e
}

// Claim commits before RPC. Expired workers never inherit a new lease token.
func (a *Application) claimPublication(ctx context.Context) (publicationIntent, bool, error) {
	var in publicationIntent
	tx, e := a.Pool.Begin(ctx)
	if e != nil {
		return in, false, e
	}
	defer tx.Rollback(context.Background())
	e = tx.QueryRow(ctx, `WITH due AS (
 SELECT actor_user_id,operation_id FROM applications.workflow_publications
 WHERE status IN ('pending','unknown') AND next_attempt_at<=clock_timestamp()
 AND (lease_until IS NULL OR lease_until<=clock_timestamp())
 ORDER BY next_attempt_at,created_at,actor_user_id,operation_id LIMIT 1 FOR UPDATE SKIP LOCKED
 ) UPDATE applications.workflow_publications p SET lease_token=gen_random_uuid(),lease_until=clock_timestamp()+interval '45 seconds',attempts=LEAST(p.attempts::bigint+1,2147483647)::integer,updated_at=clock_timestamp()
 FROM due WHERE p.actor_user_id=due.actor_user_id AND p.operation_id=due.operation_id
 RETURNING p.actor_user_id::text,p.operation_id::text,p.app_id::text,p.view_id::text,p.flow_id::text,p.version,p.version_id::text,p.bpmn_sha256,p.actor_auth_version,p.accepted_close_epoch,p.lease_token::text,p.attempts`).Scan(&in.ActorID, &in.OperationID, &in.AppID, &in.ViewID, &in.FlowID, &in.Version, &in.VersionID, &in.Hash, &in.AuthVersion, &in.CloseEpoch, &in.Token, &in.Attempts)
	if errors.Is(e, pgx.ErrNoRows) {
		return in, false, nil
	}
	if e != nil {
		return in, false, e
	}
	if e = tx.QueryRow(ctx, `SELECT bpmn_xml FROM applications.workflow_versions WHERE app_id=$1 AND flow_id=$2 AND version=$3 AND version_id=$4`, in.AppID, in.FlowID, in.Version, in.VersionID).Scan(&in.BPMN); e != nil {
		return in, false, e
	}
	if e = tx.Commit(ctx); e != nil {
		return in, false, e
	}
	return in, true, nil
}
func retrySeconds(attempts int) int {
	shift := attempts - 1
	if shift < 0 {
		shift = 0
	}
	if shift > 6 {
		shift = 6
	}
	n := 1 << shift
	if n > 60 {
		n = 60
	}
	return n
}
func (a *Application) publicationUnknown(ctx context.Context, in publicationIntent, reason string) error {
	// Accepted work survives cancellation; only bounded bookkeeping continues.
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	_, e := a.Pool.Exec(cleanup, `UPDATE applications.workflow_publications SET status='unknown',reason=$4,next_attempt_at=clock_timestamp()+$5*interval '1 second',lease_token=NULL,lease_until=NULL,updated_at=clock_timestamp() WHERE actor_user_id=$1 AND operation_id=$2 AND lease_token=$3 AND lease_until>clock_timestamp() AND status IN ('pending','unknown')`, in.ActorID, in.OperationID, in.Token, reason, retrySeconds(in.Attempts))
	return e
}
func receiptBound(in publicationIntent, r *pb.DeploymentReceipt) bool {
	h := sha256.Sum256([]byte(in.BPMN))
	return r != nil && validID(in.VersionID) && utf8.ValidString(in.BPMN) && len(in.BPMN) > 0 && len(in.BPMN) <= 1<<20 && bytes.Equal(h[:], in.Hash) && r.AppId == in.AppID && r.FlowId == in.FlowID && r.VersionId == in.VersionID && r.Version == uint64(in.Version) && r.BpmnSha256 == hex.EncodeToString(in.Hash) && strings.TrimSpace(r.EngineDeploymentId) != "" && len(r.EngineDeploymentId) <= 200 && !strings.ContainsRune(r.EngineDeploymentId, 0) && utf8.ValidString(r.EngineDeploymentId) && strings.TrimSpace(r.ProcessDefinitionId) != "" && len(r.ProcessDefinitionId) <= 512 && !strings.ContainsRune(r.ProcessDefinitionId, 0) && utf8.ValidString(r.ProcessDefinitionId)
}
func (a *Application) DispatchPublication(ctx context.Context) (bool, error) {
	if a == nil || a.Pool == nil || !a.deploymentAvailable() {
		return false, session.ErrUnavailable
	}
	in, ok, e := a.claimPublication(ctx)
	if e != nil || !ok {
		return ok, e
	}
	h := sha256.Sum256([]byte(in.BPMN))
	if !bytes.Equal(h[:], in.Hash) || !utf8.ValidString(in.BPMN) || len(in.BPMN) > 1<<20 {
		return true, a.publicationUnknown(ctx, in, "ENGINE_REPLY_INVALID")
	}
	rpc, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	looked, e := a.DeploymentClient.Lookup(rpc, &pb.LookupRequest{AppId: in.AppID, FlowId: in.FlowID, VersionId: in.VersionID, Version: uint64(in.Version), BpmnSha256: hex.EncodeToString(in.Hash)})
	var receipt *pb.DeploymentReceipt
	if e == nil {
		if looked == nil {
			return true, a.publicationUnknown(ctx, in, "ENGINE_REPLY_INVALID")
		}
		switch v := looked.Result.(type) {
		case *pb.LookupResponse_Confirmed:
			if v != nil {
				receipt = v.Confirmed
			}
		case *pb.LookupResponse_NotObserved:
			if v == nil || v.NotObserved == nil {
				return true, a.publicationUnknown(ctx, in, "ENGINE_REPLY_INVALID")
			}
			receipt, e = a.DeploymentClient.Deploy(rpc, &pb.DeployRequest{AppId: in.AppID, FlowId: in.FlowID, VersionId: in.VersionID, Version: uint64(in.Version), BpmnXml: []byte(in.BPMN)})
		default:
			return true, a.publicationUnknown(ctx, in, "ENGINE_REPLY_INVALID")
		}
	}
	if e != nil {
		reason := "DEPENDENCY_UNAVAILABLE"
		if status.Code(e) == codes.AlreadyExists || status.Code(e) == codes.FailedPrecondition {
			reason = "ENGINE_CONFLICT"
		}
		return true, a.publicationUnknown(ctx, in, reason)
	}
	if !receiptBound(in, receipt) {
		return true, a.publicationUnknown(ctx, in, "ENGINE_REPLY_INVALID")
	}
	err := a.confirmPublication(ctx, in, receipt)
	if errors.Is(err, errPublicationReceipt) {
		return true, a.publicationUnknown(ctx, in, "ENGINE_REPLY_INVALID")
	}
	return true, err
}
func (a *Application) confirmPublication(ctx context.Context, in publicationIntent, r *pb.DeploymentReceipt) error {
	tx, e := a.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(context.Background())
	lockTimeout, statementTimeout := a.Limits.LockTimeout, a.Limits.StatementTimeout
	if lockTimeout <= 0 {
		lockTimeout = time.Second
	}
	if statementTimeout <= 0 {
		statementTimeout = 5 * time.Second
	}
	if _, e = tx.Exec(ctx, `SELECT set_config('lock_timeout',$1,true),set_config('statement_timeout',$2,true)`, strconv.FormatInt(lockTimeout.Milliseconds(), 10)+"ms", strconv.FormatInt(statementTimeout.Milliseconds(), 10)+"ms"); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `SELECT personnel.lock_query_revisions()`); e != nil {
		return e
	}
	// Recheck current identity under existing authority locks, without storing a Session.
	principal := session.Principal{UserID: in.ActorID, Record: session.Record{AuthVersion: strconv.FormatInt(in.AuthVersion, 10)}}
	access, e := (&personnel.Application{}).AccessForWrite(ctx, tx, principal)
	reason := ""
	if errors.Is(e, session.ErrUnauthorized) {
		reason = "AUTHORITY_REVOKED"
	} else if e != nil {
		return e
	}
	var owner string
	if e = tx.QueryRow(ctx, `SELECT owner_user_id::text FROM applications.apps WHERE id=$1 FOR UPDATE`, in.AppID).Scan(&owner); e != nil {
		return e
	}
	var registered bool
	if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM personnel.permission_catalog WHERE code='app.'||$1::text||'.access' AND app_id=$1::text AND category='application' AND enabled)`, in.AppID).Scan(&registered); e != nil {
		return e
	}
	if !registered || reason == "" && in.ActorID != owner && !access.BootstrapAdmin {
		reason = "AUTHORITY_REVOKED"
	}
	v, e := (workflowcatalog.Catalog{}).PublicationVersionInTx(ctx, tx, in.AppID, in.FlowID, in.Version)
	if e != nil {
		return e
	}
	// App gate precedes publication row lock, as in acceptance; claim has already committed.
	var token *string
	var leaseValid bool
	var state string
	if e = tx.QueryRow(ctx, `SELECT status,lease_token::text,COALESCE(lease_until>clock_timestamp(),false) FROM applications.workflow_publications WHERE actor_user_id=$1 AND operation_id=$2 FOR UPDATE`, in.ActorID, in.OperationID).Scan(&state, &token, &leaseValid); e != nil {
		return e
	}
	if state != "pending" && state != "unknown" || token == nil || *token != in.Token || !leaseValid {
		return tx.Commit(ctx)
	}
	hash := sha256.Sum256([]byte(v.BPMN))
	if v.VersionID != in.VersionID || v.Head.ViewID != in.ViewID || !bytes.Equal(hash[:], in.Hash) || !receiptBound(in, r) {
		return errPublicationReceipt
	}
	if reason == "" && (v.Head.State == "closing" || v.CloseEpoch != in.CloseEpoch) {
		reason = "FLOW_CLOSED"
	}
	if reason == "" && !v.Compatible {
		reason = "SCHEMA_INCOMPATIBLE"
	}
	if reason == "" {
		if e = authorizeAssignees(ctx, tx, in.AppID, in.ViewID, v.Graph); errors.Is(e, errApprover) {
			reason = "APPROVER_INELIGIBLE"
		} else if e != nil {
			return e
		}
	}
	result := publicationResult{OperationID: in.OperationID, FlowID: in.FlowID, Version: in.Version, Status: "confirmed"}
	if reason != "" {
		result.Status = "blocked"
		result.Reason = &reason
	} else if v.Head.CandidateVersion != in.Version {
		result.Status = "superseded"
		reason = "CANDIDATE_SUPERSEDED"
		result.Reason = &reason
	}
	// A genuine blocked receipt is retained without activation; fresh explicit requests may reuse it.
	tag, e := tx.Exec(ctx, `INSERT INTO applications.workflow_engine_receipts(app_id,flow_id,version,version_id,bpmn_sha256,engine_deployment_id,process_definition_id) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(app_id,flow_id,version) DO NOTHING`, in.AppID, in.FlowID, in.Version, in.VersionID, in.Hash, r.EngineDeploymentId, r.ProcessDefinitionId)
	if e != nil {
		return e
	}
	if tag.RowsAffected() == 0 {
		var id, engine, definition string
		var oldHash []byte
		if e = tx.QueryRow(ctx, `SELECT version_id::text,bpmn_sha256,engine_deployment_id,process_definition_id FROM applications.workflow_engine_receipts WHERE app_id=$1 AND flow_id=$2 AND version=$3`, in.AppID, in.FlowID, in.Version).Scan(&id, &oldHash, &engine, &definition); e != nil {
			return e
		}
		if id != in.VersionID || !bytes.Equal(oldHash, in.Hash) || engine != r.EngineDeploymentId || definition != r.ProcessDefinitionId {
			return errPublicationReceipt
		}
	}
	tag, e = tx.Exec(ctx, `INSERT INTO applications.workflow_deployments(app_id,flow_id,version,deployment_id) VALUES($1,$2,$3,$4) ON CONFLICT(app_id,flow_id,version) DO NOTHING`, in.AppID, in.FlowID, in.Version, r.EngineDeploymentId)
	if e != nil {
		return e
	}
	if tag.RowsAffected() == 0 {
		var prior string
		if e = tx.QueryRow(ctx, `SELECT deployment_id FROM applications.workflow_deployments WHERE app_id=$1 AND flow_id=$2 AND version=$3`, in.AppID, in.FlowID, in.Version).Scan(&prior); e != nil {
			return e
		}
		if prior != r.EngineDeploymentId {
			return errPublicationReceipt
		}
	}
	if result.Status == "confirmed" && v.Head.CurrentVersion != in.Version {
		if v.Head.Revision >= maxSafeInteger {
			return workflowcatalog.ErrConflict
		}
		if _, e = tx.Exec(ctx, `UPDATE applications.workflow_definitions SET current_version=$3,revision=revision+1,updated_at=clock_timestamp() WHERE app_id=$1 AND id=$2`, in.AppID, in.FlowID, in.Version); e != nil {
			return e
		}
	}
	if e = publicationAudit(ctx, tx, in.ActorID, in.AppID, in.ViewID, result, result.Status, in.Token); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `UPDATE applications.workflow_publications SET status=$4,reason=$5,completed_at=clock_timestamp(),updated_at=clock_timestamp(),lease_token=NULL,lease_until=NULL WHERE actor_user_id=$1 AND operation_id=$2 AND lease_token=$3`, in.ActorID, in.OperationID, in.Token, result.Status, result.Reason); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// One worker has one in-flight call. Durable due times and leases survive restart.
func (a *Application) RunPublications(ctx context.Context) error {
	if a == nil || a.Pool == nil || !a.deploymentAvailable() {
		return session.ErrUnavailable
	}
	for {
		if e := ctx.Err(); e != nil {
			return e
		}
		worked, e := a.DispatchPublication(ctx)
		if e != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		if worked && e == nil {
			continue
		}
		timer := time.NewTimer(200 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
