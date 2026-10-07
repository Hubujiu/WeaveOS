package apprecordservice

import (
	"context"
	"errors"
	"fmt"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appschema"
	fc "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	we "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowexecution"
	wr "github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc"
	"github.com/jackc/pgx/v5/pgconn"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type rootRecoveryClient struct {
	lookups, executes atomic.Int32
	lookup            func(context.Context, fc.Command) (*wr.ExecutionConfirmed, bool, error)
	execute           func(context.Context, fc.Command, fc.ExecutionPayload) (*wr.ExecutionConfirmed, error)
}

func (c *rootRecoveryClient) Lookup(ctx context.Context, cmd fc.Command) (*wr.ExecutionConfirmed, bool, error) {
	c.lookups.Add(1)
	if c.lookup == nil {
		return nil, false, nil
	}
	return c.lookup(ctx, cmd)
}
func (c *rootRecoveryClient) Execute(ctx context.Context, cmd fc.Command, p fc.ExecutionPayload) (*wr.ExecutionConfirmed, error) {
	c.executes.Add(1)
	if c.execute == nil {
		return nil, fmt.Errorf("unexpected execution")
	}
	return c.execute(ctx, cmd, p)
}
func rootWorker(f rootProjection, client *rootRecoveryClient) *we.Worker {
	return &we.Worker{Pool: f.runtime, Client: client, RPCTimeout: 5 * time.Second, Limits: appschema.Limits{LockTimeout: time.Second, StatementTimeout: 5 * time.Second}}
}
func rootWorkerReply(t *testing.T, f rootProjection, c fc.Command) *wr.ExecutionConfirmed {
	t.Helper()
	r, b := f.receipt(t, c, "active", "", f.task(t, f.first, f.actor, 1))
	decoded, e := fc.DecodeExecutionResult(c, r, b)
	if e != nil {
		t.Fatal(e)
	}
	return &wr.ExecutionConfirmed{Receipt: r, Result: decoded, ResultBytes: b}
}
func rootWorkerDue(t *testing.T, f rootProjection, c fc.Command) {
	t.Helper()
	if _, e := f.owner.Exec(f.ctx, "UPDATE applications.workflow_dispatch SET next_attempt_at=statement_timestamp()-interval '1 second' WHERE command_id=$1", c.CommandID); e != nil {
		t.Fatal(e)
	}
}
func rootWorkerPending(t *testing.T, f rootProjection, c fc.Command, reason string) {
	t.Helper()
	f.state(t, "starting", 0, 0, 0, 0)
	f.ledger(t, c, "pending", 1, 1)
	var attempts int
	var got *string
	var cleared, advanced bool
	e := f.owner.QueryRow(f.ctx, "SELECT attempts,last_error,lease_token IS NULL AND lease_until IS NULL,next_attempt_at>created_at FROM applications.workflow_dispatch WHERE command_id=$1", c.CommandID).Scan(&attempts, &got, &cleared, &advanced)
	if e != nil || attempts < 1 || got == nil || *got != reason || !cleared || !advanced {
		t.Fatalf("unsafe retry metadata: %d %v %v %v %v", attempts, got, cleared, advanced, e)
	}
}
func TestRootRecoveryWorkerExecuteAndProjectWithoutDatabaseTransactionAcrossRPC(t *testing.T) {
	f := rootProjectionFixture(t)
	cmd, payload := rootRecoveryAccept(t, f)
	reply := rootWorkerReply(t, f, cmd)
	client := &rootRecoveryClient{}
	client.lookup = func(ctx context.Context, c fc.Command) (*wr.ExecutionConfirmed, bool, error) {
		if c != cmd || f.runtime.Stat().AcquiredConns() != 0 {
			return nil, false, fmt.Errorf("wrong identity or database transaction spans Lookup")
		}
		if _, ok := ctx.Deadline(); !ok {
			return nil, false, fmt.Errorf("RPC must be bounded")
		}
		return nil, false, nil
	}
	client.execute = func(ctx context.Context, c fc.Command, p fc.ExecutionPayload) (*wr.ExecutionConfirmed, error) {
		want, _ := fc.EncodeExecutionPayload(cmd.Action, payload)
		got, _ := fc.EncodeExecutionPayload(c.Action, p)
		if c != cmd || string(got) != string(want) || f.runtime.Stat().AcquiredConns() != 0 {
			return nil, fmt.Errorf("changed request or database transaction spans Execute")
		}
		return reply, nil
	}
	worked, e := rootWorker(f, client).DispatchOne(f.ctx)
	if e != nil || !worked {
		t.Fatalf("dispatch: %v %v", worked, e)
	}
	f.state(t, "active", 1, 1, 1, 1)
	f.ledger(t, cmd, "success", 0, 0)
	var body []byte
	if e = f.owner.QueryRow(f.ctx, "SELECT result_bytes FROM applications.workflow_execution_events WHERE command_id=$1", cmd.CommandID).Scan(&body); e != nil || string(body) != string(reply.ResultBytes) {
		t.Fatalf("original RPC evidence lost: %v", e)
	}
	if client.lookups.Load() != 1 || client.executes.Load() != 1 {
		t.Fatal("unexpected transport retries")
	}
	worked, e = rootWorker(f, client).DispatchOne(f.ctx)
	if e != nil || worked {
		t.Fatalf("completed work remained due: %v %v", worked, e)
	}
}
func TestRootRecoveryWorkerLookupConfirmedNeverReexecutes(t *testing.T) {
	f := rootProjectionFixture(t)
	cmd, _ := rootRecoveryAccept(t, f)
	reply := rootWorkerReply(t, f, cmd)
	client := &rootRecoveryClient{lookup: func(context.Context, fc.Command) (*wr.ExecutionConfirmed, bool, error) { return reply, true, nil }}
	if worked, e := rootWorker(f, client).DispatchOne(f.ctx); e != nil || !worked {
		t.Fatalf("lookup apply: %v", e)
	}
	f.state(t, "active", 1, 1, 1, 1)
	f.ledger(t, cmd, "success", 0, 0)
	if client.executes.Load() != 0 {
		t.Fatal("confirmed engine result was reexecuted")
	}
}
func TestRootRecoveryWorkerLookupFailureStaysPending(t *testing.T) {
	f := rootProjectionFixture(t)
	cmd, _ := rootRecoveryAccept(t, f)
	client := &rootRecoveryClient{lookup: func(context.Context, fc.Command) (*wr.ExecutionConfirmed, bool, error) {
		return nil, false, context.DeadlineExceeded
	}}
	worked, e := rootWorker(f, client).DispatchOne(f.ctx)
	if !worked || !errors.Is(e, context.DeadlineExceeded) {
		t.Fatalf("unknown lookup was hidden: %v", e)
	}
	rootWorkerPending(t, f, cmd, "DEPENDENCY_UNAVAILABLE")
	if client.executes.Load() != 0 {
		t.Fatal("failed lookup was treated as absent")
	}
}
func TestRootRecoveryWorkerLostExecuteResponseRestartsWithSameIdentity(t *testing.T) {
	f := rootProjectionFixture(t)
	cmd, payload := rootRecoveryAccept(t, f)
	reply := rootWorkerReply(t, f, cmd)
	first := &rootRecoveryClient{execute: func(context.Context, fc.Command, fc.ExecutionPayload) (*wr.ExecutionConfirmed, error) {
		return nil, context.DeadlineExceeded
	}}
	if worked, e := rootWorker(f, first).DispatchOne(f.ctx); !worked || !errors.Is(e, context.DeadlineExceeded) {
		t.Fatalf("lost response not kept unknown: %v", e)
	}
	rootWorkerPending(t, f, cmd, "DEPENDENCY_UNAVAILABLE")
	rootWorkerDue(t, f, cmd)
	second := &rootRecoveryClient{lookup: func(_ context.Context, c fc.Command) (*wr.ExecutionConfirmed, bool, error) {
		if c != cmd {
			return nil, false, fmt.Errorf("restart invented a new command")
		}
		return reply, true, nil
	}}
	if worked, e := rootWorker(f, second).DispatchOne(f.ctx); e != nil || !worked {
		t.Fatalf("restart did not recover: %v", e)
	}
	f.state(t, "active", 1, 1, 1, 1)
	f.ledger(t, cmd, "success", 0, 0)
	got := rootRecoveryPayload(t, f, cmd)
	a, _ := fc.EncodeExecutionPayload(cmd.Action, got)
	b, _ := fc.EncodeExecutionPayload(cmd.Action, payload)
	if string(a) != string(b) || first.executes.Load() != 1 || second.executes.Load() != 0 {
		t.Fatal("restart changed original request or repeated effect")
	}
}
func TestRootRecoveryWorkerRejectsMalformedOrContradictoryReplies(t *testing.T) {
	for _, kind := range []string{"nil-confirmed", "contradictory-absent", "wrong-body-hash", "nil-execute"} {
		t.Run(kind, func(t *testing.T) {
			f := rootProjectionFixture(t)
			cmd, _ := rootRecoveryAccept(t, f)
			reply := rootWorkerReply(t, f, cmd)
			client := &rootRecoveryClient{}
			switch kind {
			case "nil-confirmed":
				client.lookup = func(context.Context, fc.Command) (*wr.ExecutionConfirmed, bool, error) { return nil, true, nil }
			case "contradictory-absent":
				client.lookup = func(context.Context, fc.Command) (*wr.ExecutionConfirmed, bool, error) { return reply, false, nil }
			case "wrong-body-hash":
				reply.ResultBytes[0] ^= 1
				client.lookup = func(context.Context, fc.Command) (*wr.ExecutionConfirmed, bool, error) { return reply, true, nil }
			case "nil-execute":
				client.execute = func(context.Context, fc.Command, fc.ExecutionPayload) (*wr.ExecutionConfirmed, error) {
					return nil, nil
				}
			}
			worked, e := rootWorker(f, client).DispatchOne(f.ctx)
			if !worked || e == nil {
				t.Fatal("unproven result reported success")
			}
			rootWorkerPending(t, f, cmd, "ENGINE_REPLY_INVALID")
			if kind != "nil-execute" && client.executes.Load() != 0 {
				t.Fatal("invalid Lookup was treated as not observed")
			}
		})
	}
}
func TestRootRecoveryWorkerProjectionFailureKeepsFenceThenRecovers(t *testing.T) {
	f := rootProjectionFixture(t)
	cmd, _ := rootRecoveryAccept(t, f)
	reply := rootWorkerReply(t, f, cmd)
	fn := "root_worker_fault_" + fmt.Sprintf("%x", cmd.CommandID[:8])
	query := fmt.Sprintf("CREATE FUNCTION applications.%s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'root worker projection fault' USING ERRCODE='P0001'; END $$; CREATE TRIGGER %s AFTER INSERT ON applications.workflow_execution_events FOR EACH ROW WHEN (NEW.command_id='%s'::uuid) EXECUTE FUNCTION applications.%s()", fn, fn, cmd.CommandID, fn)
	if _, e := f.owner.Exec(f.ctx, query); e != nil {
		t.Fatal(e)
	}
	cleanup := func() {
		_, _ = f.owner.Exec(context.Background(), fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON applications.workflow_execution_events; DROP FUNCTION IF EXISTS applications.%s()", fn, fn))
	}
	t.Cleanup(cleanup)
	client := &rootRecoveryClient{lookup: func(context.Context, fc.Command) (*wr.ExecutionConfirmed, bool, error) { return reply, true, nil }}
	worked, e := rootWorker(f, client).DispatchOne(f.ctx)
	var fault *pgconn.PgError
	if !worked || !errors.As(e, &fault) || fault.Code != "P0001" {
		t.Fatalf("did not reach actual projection fault: %v", e)
	}
	rootWorkerPending(t, f, cmd, "APPLICATION_CONFIRMATION_FAILED")
	cleanup()
	rootWorkerDue(t, f, cmd)
	if worked, e = rootWorker(f, client).DispatchOne(f.ctx); e != nil || !worked {
		t.Fatalf("confirmed result did not recover: %v", e)
	}
	f.state(t, "active", 1, 1, 1, 1)
	f.ledger(t, cmd, "success", 0, 0)
	if client.executes.Load() != 0 {
		t.Fatal("projection retry reexecuted engine action")
	}
}
func TestRootRecoveryWorkerClaimPreventsDuplicateLiveDispatch(t *testing.T) {
	f := rootProjectionFixture(t)
	cmd, _ := rootRecoveryAccept(t, f)
	reply := rootWorkerReply(t, f, cmd)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	var arrivals atomic.Int32
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	client := &rootRecoveryClient{lookup: func(ctx context.Context, _ fc.Command) (*wr.ExecutionConfirmed, bool, error) {
		if arrivals.Add(1) != 1 {
			return nil, false, fmt.Errorf("duplicate live dispatch")
		}
		close(entered)
		select {
		case <-release:
			return reply, true, nil
		case <-ctx.Done():
			return nil, false, ctx.Err()
		}
	}}
	type result struct {
		worked bool
		err    error
	}
	done := make(chan result, 1)
	go func() { w, e := rootWorker(f, client).DispatchOne(f.ctx); done <- result{w, e} }()
	select {
	case <-entered:
	case early := <-done:
		t.Fatalf("returned before RPC: %v", early.err)
	case <-time.After(5 * time.Second):
		t.Fatal("first worker did not claim")
	}
	for i := 0; i < 8; i++ {
		if worked, e := rootWorker(f, client).DispatchOne(f.ctx); e != nil || worked {
			t.Fatalf("live lease was concurrently dispatched: %v %v", worked, e)
		}
	}
	once.Do(func() { close(release) })
	var got result
	select {
	case got = <-done:
	case <-time.After(12 * time.Second):
		t.Fatal("worker did not finish")
	}
	if !got.worked || got.err != nil {
		t.Fatalf("first dispatch: %v", got.err)
	}
	f.ledger(t, cmd, "success", 0, 0)
	if client.lookups.Load() != 1 {
		t.Fatal("one live lease reached transport more than once")
	}
}
func TestRootRecoveryWorkerExpiredLeaseCannotAffectLaterFence(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		t.Run(fmt.Sprint(conflict), func(t *testing.T) {
			f := rootProjectionFixture(t)
			cmd, _ := rootRecoveryAccept(t, f)
			reply := rootWorkerReply(t, f, cmd)
			late := *reply
			if conflict {
				late.Receipt.ProofID = recordOperationID(t, f.recordFixture)
			}
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			var calls atomic.Int32
			t.Cleanup(func() { once.Do(func() { close(release) }) })
			client := &rootRecoveryClient{lookup: func(ctx context.Context, _ fc.Command) (*wr.ExecutionConfirmed, bool, error) {
				if calls.Add(1) == 1 {
					close(entered)
					select {
					case <-release:
						return &late, true, nil
					case <-ctx.Done():
						return nil, false, ctx.Err()
					}
				}
				return reply, true, nil
			}}
			type result struct {
				worked bool
				err    error
			}
			done := make(chan result, 1)
			go func() { w, e := rootWorker(f, client).DispatchOne(f.ctx); done <- result{w, e} }()
			select {
			case <-entered:
			case early := <-done:
				t.Fatalf("returned before RPC: %v", early.err)
			case <-time.After(5 * time.Second):
				t.Fatal("first lease never reached transport")
			}
			if _, e := f.owner.Exec(f.ctx, "UPDATE applications.workflow_dispatch SET lease_until=statement_timestamp()-interval '1 second',next_attempt_at=statement_timestamp()-interval '1 second' WHERE command_id=$1", cmd.CommandID); e != nil {
				t.Fatal(e)
			}
			if worked, e := rootWorker(f, client).DispatchOne(f.ctx); e != nil || !worked {
				t.Fatalf("expired claim was not recovered: %v", e)
			}
			next, _ := f.accept(t, "agree", reply.Result.Tasks[0].ID, "", 1, 1)
			once.Do(func() { close(release) })
			var old result
			select {
			case old = <-done:
			case <-time.After(12 * time.Second):
				t.Fatal("late worker did not finish")
			}
			if !old.worked || conflict && !errors.Is(old.err, fc.ErrConflict) || !conflict && old.err != nil {
				t.Fatalf("late result mismatch: %v", old.err)
			}
			f.state(t, "active", 1, 1, 1, 1)
			f.ledger(t, next, "pending", 1, 1)
			if client.executes.Load() != 0 {
				t.Fatal("confirmed old result was reexecuted")
			}
		})
	}
}
func TestRootRecoveryWorkerAcceptedCommandSurvivesAuthorityRevocation(t *testing.T) {
	f := rootProjectionFixture(t)
	cmd, _ := rootRecoveryAccept(t, f)
	reply := rootWorkerReply(t, f, cmd)
	if _, e := f.owner.Exec(f.ctx, "UPDATE auth.users SET status='disabled',auth_version=auth_version+1 WHERE id=$1", f.actor); e != nil {
		t.Fatal(e)
	}
	client := &rootRecoveryClient{lookup: func(context.Context, fc.Command) (*wr.ExecutionConfirmed, bool, error) { return reply, true, nil }}
	if worked, e := rootWorker(f, client).DispatchOne(f.ctx); e != nil || !worked {
		t.Fatalf("already accepted command was abandoned: %v", e)
	}
	f.ledger(t, cmd, "success", 0, 0)
}
func TestRootRecoveryWorkerLegacyQueueAndFutureDueAreNotExecuted(t *testing.T) {
	legacy := rootProjectionFixture(t)
	legacy.accept(t, "start", "", "", 0, 0)
	future := rootProjectionFixture(t)
	cmd, _ := rootRecoveryAccept(t, future)
	if _, e := future.owner.Exec(future.ctx, "UPDATE applications.workflow_dispatch SET next_attempt_at=statement_timestamp()+interval '1 hour' WHERE command_id=$1", cmd.CommandID); e != nil {
		t.Fatal(e)
	}
	client := &rootRecoveryClient{}
	if worked, e := rootWorker(future, client).DispatchOne(future.ctx); e != nil || worked {
		t.Fatalf("legacy/future queue was selected: %v", e)
	}
	if client.lookups.Load() != 0 || client.executes.Load() != 0 {
		t.Fatal("legacy or delayed input reached transport")
	}
}
func TestRootRecoveryWorkerCorruptStoredPayloadFailsBeforeRPC(t *testing.T) {
	f := rootProjectionFixture(t)
	cmd, p := rootRecoveryAccept(t, f)
	raw, _ := fc.EncodeExecutionPayload(cmd.Action, p)
	raw[8] ^= 1
	if _, e := f.owner.Exec(f.ctx, "UPDATE applications.workflow_commands SET execution_payload=$2 WHERE command_id=$1", cmd.CommandID, raw); e != nil {
		t.Fatal(e)
	}
	client := &rootRecoveryClient{}
	if worked, e := rootWorker(f, client).DispatchOne(f.ctx); !worked || e == nil {
		t.Fatalf("corrupt accepted input was not isolated: %v", e)
	}
	rootWorkerPending(t, f, cmd, "COMMAND_INVALID")
	if client.lookups.Load() != 0 || client.executes.Load() != 0 {
		t.Fatal("corrupt source reached engine")
	}
}

func TestRootRecoveryWorkerOtherRecordContinuesDuringUnknownRPC(t *testing.T) {
	first := rootProjectionFixture(t)
	a, _ := rootRecoveryAccept(t, first)
	ra := rootWorkerReply(t, first, a)
	second := rootProjectionFixture(t)
	b, _ := rootRecoveryAccept(t, second)
	rb := rootWorkerReply(t, second, b)
	if _, e := first.owner.Exec(first.ctx, "UPDATE applications.workflow_dispatch SET next_attempt_at=statement_timestamp()-CASE WHEN command_id=$1 THEN interval '2 seconds' ELSE interval '1 second' END WHERE command_id IN ($1,$2)", a.CommandID, b.CommandID); e != nil {
		t.Fatal(e)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	var firstCalls atomic.Int32
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	client := &rootRecoveryClient{lookup: func(ctx context.Context, c fc.Command) (*wr.ExecutionConfirmed, bool, error) {
		if c.CommandID == b.CommandID {
			return rb, true, nil
		}
		if c.CommandID != a.CommandID {
			return nil, false, fmt.Errorf("unexpected command")
		}
		if firstCalls.Add(1) != 1 {
			return nil, false, fmt.Errorf("live first command was dispatched twice")
		}
		close(entered)
		select {
		case <-release:
			return ra, true, nil
		case <-ctx.Done():
			return nil, false, ctx.Err()
		}
	}}
	type result struct {
		worked bool
		err    error
	}
	done := make(chan result, 1)
	go func() { w, e := rootWorker(first, client).DispatchOne(first.ctx); done <- result{w, e} }()
	select {
	case <-entered:
	case early := <-done:
		t.Fatalf("returned before RPC: %v", early.err)
	case <-time.After(5 * time.Second):
		t.Fatal("first command never reached RPC")
	}
	if worked, e := rootWorker(first, client).DispatchOne(first.ctx); e != nil || !worked {
		t.Fatalf("other record was blocked by network call: %v", e)
	}
	second.state(t, "active", 1, 1, 1, 1)
	second.ledger(t, b, "success", 0, 0)
	first.state(t, "starting", 0, 0, 0, 0)
	first.ledger(t, a, "pending", 1, 1)
	once.Do(func() { close(release) })
	var got result
	select {
	case got = <-done:
	case <-time.After(12 * time.Second):
		t.Fatal("first command did not finish")
	}
	if !got.worked || got.err != nil {
		t.Fatalf("first command failed after release: %v", got.err)
	}
	first.state(t, "active", 1, 1, 1, 1)
}

func TestRootRecoveryWorkerConfirmedNoEffectUsesExistingAtomicProjection(t *testing.T) {
	f := rootProjectionFixture(t)
	cmd, _ := rootRecoveryAccept(t, f)
	receipt, body := f.receipt(t, cmd, "unchanged", "cancelled")
	decoded, e := fc.DecodeExecutionResult(cmd, receipt, body)
	if e != nil {
		t.Fatal(e)
	}
	reply := &wr.ExecutionConfirmed{Receipt: receipt, Result: decoded, ResultBytes: body}
	client := &rootRecoveryClient{lookup: func(context.Context, fc.Command) (*wr.ExecutionConfirmed, bool, error) { return reply, true, nil }}
	if worked, e := rootWorker(f, client).DispatchOne(f.ctx); e != nil || !worked {
		t.Fatalf("proven no-effect was not applied: %v", e)
	}
	f.state(t, "no_effect", 0, 1, 0, 0)
	f.ledger(t, cmd, "no_effect", 0, 0)
	if client.executes.Load() != 0 {
		t.Fatal("durable no-effect was reexecuted")
	}
}
