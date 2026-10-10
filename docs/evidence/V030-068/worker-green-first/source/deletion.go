package appworkflows

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowcatalog"
	pb "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc/pb"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
)

type DeletionClient interface {
	DeleteFlow(context.Context, *pb.FlowDeletionRequest) (*pb.FlowDeletionReceipt, error)
	LookupFlowDeletion(context.Context, *pb.FlowDeletionRequest) (*pb.FlowDeletionLookupResponse, error)
}
type deletionIntent struct {
	AppID, TableID, FlowID, OperationID, Token string
	Attempts                                   int
	Confirmed                                  bool
}

func (a *Application) deletionAvailable() bool {
	if a == nil || a.Pool == nil || a.DeletionClient == nil {
		return false
	}
	v := reflect.ValueOf(a.DeletionClient)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !v.IsNil()
	}
	return true
}
func (a *Application) beginDeletion(ctx context.Context) (pgx.Tx, error) {
	tx, e := a.Pool.Begin(ctx)
	if e != nil {
		return nil, e
	}
	lock, statement := a.Limits.LockTimeout, a.Limits.StatementTimeout
	if lock <= 0 {
		lock = time.Second
	}
	if statement <= 0 {
		statement = 5 * time.Second
	}
	if _, e = tx.Exec(ctx, `SELECT set_config('lock_timeout',$1,true),set_config('statement_timeout',$2,true)`, strconv.FormatInt(lock.Milliseconds(), 10)+"ms", strconv.FormatInt(statement.Milliseconds(), 10)+"ms"); e != nil {
		_ = tx.Rollback(context.Background())
		return nil, e
	}
	return tx, nil
}
func (a *Application) claimDeletion(ctx context.Context) (deletionIntent, bool, error) {
	var in deletionIntent
	tx, e := a.beginDeletion(ctx)
	if e != nil {
		return in, false, e
	}
	defer tx.Rollback(context.Background())
	e = tx.QueryRow(ctx, `WITH due AS (SELECT flow_id FROM applications.workflow_deletions
 WHERE status IN ('pending','unknown') AND next_attempt_at<=clock_timestamp() AND (lease_until IS NULL OR lease_until<=clock_timestamp())
 ORDER BY next_attempt_at,created_at,flow_id LIMIT 1 FOR UPDATE SKIP LOCKED)
 UPDATE applications.workflow_deletions d SET lease_token=gen_random_uuid(),lease_until=clock_timestamp()+interval '45 seconds',attempts=LEAST(d.attempts::bigint+1,2147483647)::integer,updated_at=clock_timestamp()
 FROM due WHERE d.flow_id=due.flow_id RETURNING d.app_id::text,d.table_id::text,d.flow_id::text,d.operation_id::text,d.lease_token::text,d.attempts,d.engine_deleted_at IS NOT NULL`).Scan(&in.AppID, &in.TableID, &in.FlowID, &in.OperationID, &in.Token, &in.Attempts, &in.Confirmed)
	if errors.Is(e, pgx.ErrNoRows) {
		return in, false, nil
	}
	if e != nil {
		return in, false, e
	}
	if e = tx.Commit(ctx); e != nil {
		return in, false, e
	}
	return in, true, nil
}

// App -> table -> definition -> intent, matching normal management writes.
func lockDeletion(ctx context.Context, tx pgx.Tx, in deletionIntent) (bool, error) {
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`SELECT 1 FROM applications.apps WHERE id=$1 FOR UPDATE`, []any{in.AppID}},
		{`SELECT 1 FROM applications.logical_tables WHERE app_id=$1 AND id=$2 FOR UPDATE`, []any{in.AppID, in.TableID}},
		{`SELECT 1 FROM applications.workflow_definitions WHERE app_id=$1 AND id=$2 FOR UPDATE`, []any{in.AppID, in.FlowID}},
	} {
		if _, e := tx.Exec(ctx, q.sql, q.args...); e != nil {
			return false, e
		}
	}
	var valid bool
	e := tx.QueryRow(ctx, `SELECT status IN ('pending','unknown') AND lease_token=$3 AND lease_until>clock_timestamp() FROM applications.workflow_deletions WHERE app_id=$1 AND flow_id=$2 AND operation_id=$4 FOR UPDATE`, in.AppID, in.FlowID, in.Token, in.OperationID).Scan(&valid)
	if errors.Is(e, pgx.ErrNoRows) {
		return false, nil
	}
	return valid, e
}
func (a *Application) deletionReady(ctx context.Context, in deletionIntent) (bool, bool, error) {
	tx, e := a.beginDeletion(ctx)
	if e != nil {
		return false, false, e
	}
	defer tx.Rollback(context.Background())
	valid, e := lockDeletion(ctx, tx, in)
	if e != nil || !valid {
		return valid, false, e
	}
	var busy bool
	e = tx.QueryRow(ctx, `SELECT applications.workflow_deletion_has_work($1,$2)`, in.AppID, in.FlowID).Scan(&busy)
	if e != nil {
		return false, false, e
	}
	return true, !busy, tx.Commit(ctx)
}
func (a *Application) rescheduleDeletion(ctx context.Context, in deletionIntent, state, reason string) error {
	_, e := a.Pool.Exec(ctx, `UPDATE applications.workflow_deletions SET status=$5,reason=$6,next_attempt_at=clock_timestamp()+$7*interval '1 second',lease_token=NULL,lease_until=NULL,updated_at=clock_timestamp()
 WHERE app_id=$1 AND flow_id=$2 AND operation_id=$3 AND lease_token=$4 AND lease_until>clock_timestamp() AND status IN ('pending','unknown')`, in.AppID, in.FlowID, in.OperationID, in.Token, state, reason, retrySeconds(in.Attempts))
	return e
}
func boundDeletion(in deletionIntent, r *pb.FlowDeletionReceipt) bool {
	return r != nil && proto.Size(r) <= 1024 && r.AppId == in.AppID && r.FlowId == in.FlowID && r.OperationId == in.OperationID && r.DeletedVersions <= uint64(maxSafeInteger) && r.DeletedAtSeconds >= 1 && r.DeletedAtSeconds <= 253402300799 && r.DeletedAtNanos >= 0 && r.DeletedAtNanos <= 999999999 && r.DeletedAtNanos%1000 == 0
}
func (a *Application) recordDeletionReceipt(ctx context.Context, in deletionIntent, r *pb.FlowDeletionReceipt) (bool, error) {
	tx, e := a.beginDeletion(ctx)
	if e != nil {
		return false, e
	}
	defer tx.Rollback(context.Background())
	valid, e := lockDeletion(ctx, tx, in)
	if e != nil || !valid {
		return valid, e
	}
	var oldTime *time.Time
	var oldCount *int64
	if e = tx.QueryRow(ctx, `SELECT engine_deleted_at,engine_deleted_versions FROM applications.workflow_deletions WHERE flow_id=$1`, in.FlowID).Scan(&oldTime, &oldCount); e != nil {
		return false, e
	}
	stamp := time.Unix(r.DeletedAtSeconds, int64(r.DeletedAtNanos)).UTC()
	if oldTime != nil {
		if oldCount == nil || *oldCount != int64(r.DeletedVersions) || !oldTime.Equal(stamp) {
			return false, workflowcatalog.ErrConflict
		}
		return true, tx.Commit(ctx)
	}
	tag, e := tx.Exec(ctx, `UPDATE applications.workflow_deletions SET engine_deleted_at=$5,engine_deleted_versions=$6,updated_at=clock_timestamp() WHERE app_id=$1 AND flow_id=$2 AND operation_id=$3 AND lease_token=$4 AND lease_until>clock_timestamp()`, in.AppID, in.FlowID, in.OperationID, in.Token, stamp, int64(r.DeletedVersions))
	if e != nil {
		return false, e
	}
	if tag.RowsAffected() != 1 {
		return false, nil
	}
	return true, tx.Commit(ctx)
}
func (a *Application) completeDeletion(ctx context.Context, in deletionIntent) error {
	tx, e := a.beginDeletion(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(context.Background())
	var completed bool
	if e = tx.QueryRow(ctx, `SELECT applications.complete_workflow_deletion($1,$2,$3,$4)`, in.AppID, in.FlowID, in.OperationID, in.Token).Scan(&completed); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// DispatchDeletion handles one durable intent. No database transaction survives
// an RPC. The authenticated engine independently verifies drain and original IDs.
func (a *Application) DispatchDeletion(ctx context.Context) (bool, error) {
	if ctx == nil || !a.deletionAvailable() {
		return false, workflowcatalog.ErrNotReady
	}
	in, found, e := a.claimDeletion(ctx)
	if e != nil || !found {
		return false, e
	}
	valid, ready, e := a.deletionReady(ctx, in)
	if e != nil {
		return true, a.rescheduleDeletion(ctx, in, "unknown", "cleanup_retry")
	}
	if !valid {
		return true, nil
	}
	if !ready {
		return true, a.rescheduleDeletion(ctx, in, "pending", "waiting_work")
	}
	if !in.Confirmed {
		rpc, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		q := &pb.FlowDeletionRequest{AppId: in.AppID, FlowId: in.FlowID, OperationId: in.OperationID}
		observed, err := a.DeletionClient.LookupFlowDeletion(rpc, q)
		if err != nil {
			return true, a.rescheduleDeletion(ctx, in, "unknown", "engine_unavailable")
		}
		var receipt *pb.FlowDeletionReceipt
		if observed == nil || proto.Size(observed) > 1024 {
			return true, a.rescheduleDeletion(ctx, in, "unknown", "receipt_invalid")
		}
		switch result := observed.Result.(type) {
		case *pb.FlowDeletionLookupResponse_Confirmed:
			if result != nil {
				receipt = result.Confirmed
			}
		case *pb.FlowDeletionLookupResponse_NotObserved:
			if result == nil || result.NotObserved == nil {
				return true, a.rescheduleDeletion(ctx, in, "unknown", "receipt_invalid")
			}
			receipt, err = a.DeletionClient.DeleteFlow(rpc, q)
		default:
			return true, a.rescheduleDeletion(ctx, in, "unknown", "receipt_invalid")
		}
		if err != nil {
			return true, a.rescheduleDeletion(ctx, in, "unknown", "engine_unavailable")
		}
		if !boundDeletion(in, receipt) {
			return true, a.rescheduleDeletion(ctx, in, "unknown", "receipt_invalid")
		}
		valid, e = a.recordDeletionReceipt(ctx, in, receipt)
		if e != nil {
			return true, a.rescheduleDeletion(ctx, in, "unknown", "cleanup_retry")
		}
		if !valid {
			return true, nil
		}
	}
	if e = a.completeDeletion(ctx, in); e != nil {
		return true, a.rescheduleDeletion(ctx, in, "unknown", "cleanup_retry")
	}
	return true, nil
}
