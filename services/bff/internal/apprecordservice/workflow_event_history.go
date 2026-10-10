package apprecordservice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	fc "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

type WorkflowEventHistoryRequest struct {
	AppID, ViewID, RecordID, PageToken string
	PageSize                           int
}
type WorkflowEventHistoryResult struct {
	Items         []WorkflowEventSummary
	HasMore       bool
	NextPageToken *string
}
type eventHistoryCursor struct {
	Actor, App, Table, View, Record string
	Policy, Schema                  int64
	Size                            int
	AfterTime                       time.Time
	AfterID                         string
	Expires                         int64
}

func (c eventHistoryCursor) resource() string {
	return "workflow-history:" + c.App + ":" + c.View + ":" + c.Record
}
func validEventHistoryCursor(c eventHistoryCursor) bool {
	for _, id := range []string{c.Actor, c.App, c.Table, c.View, c.Record, c.AfterID} {
		if !workflowID(id) {
			return false
		}
	}
	return workflowPositive(c.Policy) && workflowPositive(c.Schema) && c.Size >= 1 && c.Size <= 100 && !c.AfterTime.IsZero() && c.Expires > 0
}
func eventHistoryFingerprint(raw []byte) string {
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}
func newWorkflowEventHistoryStore(client redis.UniversalClient, generation string) *querycontext.Store {
	return querycontext.NewStore(client, "workflow-history", generation, querycontext.Policy{Validate: func(m querycontext.Metadata) bool {
		var c eventHistoryCursor
		return len(m.Criteria) <= 2048 && closedJSON(m.Criteria, &c) && validEventHistoryCursor(c) && m.View == c.resource() && m.Total == 0 && m.Fingerprint == eventHistoryFingerprint(m.Criteria) && string(m.Revision) == "{}"
	}})
}
func (s *Service) ReadWorkflowEventHistory(ctx context.Context, p session.Principal, q WorkflowEventHistoryRequest) (WorkflowEventHistoryResult, error) {
	empty := WorkflowEventHistoryResult{}
	if s == nil || s.Pool == nil || s.WorkflowEventHistory == nil {
		return empty, ErrUnavailable
	}
	if captureNilPort(ctx) {
		return empty, applications.ErrInvalid
	}
	for _, id := range []string{q.AppID, q.ViewID, q.RecordID} {
		if !workflowID(id) {
			return empty, applications.ErrInvalid
		}
	}
	if q.PageSize == 0 {
		q.PageSize = 20
	}
	if q.PageSize < 1 || q.PageSize > 100 {
		return empty, applications.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	a, err := s.beginWorkflowHistoryRead(ctx, p, WorkflowEventRequest{AppID: q.AppID, ViewID: q.ViewID, RecordID: q.RecordID})
	if err != nil {
		return empty, err
	}
	defer a.tx.Rollback(context.Background())
	want := eventHistoryCursor{Actor: p.UserID, App: q.AppID, Table: a.facts.TableID, View: q.ViewID, Record: q.RecordID, Policy: a.facts.App.PolicyRevision, Schema: a.facts.SchemaVersion, Size: q.PageSize}
	var afterTime *time.Time
	var afterID *string
	if q.PageToken != "" {
		m, e := s.WorkflowEventHistory.Load(ctx, p.SessionRef, q.PageToken)
		if e != nil {
			return empty, e
		}
		var got eventHistoryCursor
		if !closedJSON(m.Criteria, &got) || !validEventHistoryCursor(got) || got.Expires <= time.Now().Unix() {
			return empty, querycontext.ErrExpired
		}
		boundary, expiry := got.AfterTime, got.Expires
		id := got.AfterID
		got.AfterTime = time.Time{}
		got.AfterID = ""
		got.Expires = 0
		if got != want || m.View != want.resource() || expiry <= 0 {
			return empty, querycontext.ErrExpired
		}
		afterTime = &boundary
		afterID = &id
	}
	items, err := eventHistoryPage(ctx, a.tx, q, a.facts.TableID, afterTime, afterID)
	if err != nil {
		return empty, err
	}
	out := WorkflowEventHistoryResult{Items: items, HasMore: len(items) > q.PageSize}
	if out.HasMore {
		out.Items = out.Items[:q.PageSize]
	}
	if err = a.tx.Commit(ctx); err != nil {
		return empty, ErrUnavailable
	}
	if out.HasMore {
		last := out.Items[len(out.Items)-1]
		want.AfterTime, err = time.Parse(time.RFC3339Nano, last.OccurredAt)
		if err != nil {
			return empty, ErrUnavailable
		}
		want.AfterID = last.ID
		want.Expires = time.Now().Add(10 * time.Minute).Unix()
		raw, e := json.Marshal(want)
		if e != nil {
			return empty, ErrUnavailable
		}
		token, e := s.WorkflowEventHistory.Create(ctx, p.SessionRef, querycontext.Metadata{View: want.resource(), Criteria: raw, Revision: json.RawMessage(`{}`), Total: 0, Fingerprint: eventHistoryFingerprint(raw), ProtocolVersion: 1})
		if e != nil {
			return empty, e
		}
		out.NextPageToken = &token
	}
	return out, nil
}

type historyEventRow struct {
	event                      WorkflowEventSummary
	sourceView, proof, version string
	task                       *string
	epoch                      int64
	definition                 int64
	at                         time.Time
}

func eventHistoryPage(ctx context.Context, tx pgx.Tx, q WorkflowEventHistoryRequest, table string, afterTime *time.Time, afterID *string) ([]WorkflowEventSummary, error) {
	rows, err := tx.Query(ctx, `SELECT e.command_id::text,e.instance_id::text,e.flow_id::text,e.view_id::text,e.definition_version,
 e.actor_id::text,e.action,e.outcome,e.sequence,e.schema_version,e.record_version,e.created_at,e.proof_id::text,e.version_id::text,e.task_id::text,e.task_epoch,e.node_id::text,e.target_node_id::text,e.flow_name,e.flow_name_source
 FROM applications.workflow_execution_events e
 WHERE e.app_id=$1::uuid AND e.table_id=$2::uuid AND e.record_id=$3::uuid
 AND ($4::timestamptz IS NULL OR (e.created_at,e.command_id)<($4,$5::uuid))
 ORDER BY e.created_at DESC,e.command_id DESC LIMIT $6`, q.AppID, table, q.RecordID, afterTime, afterID, q.PageSize+1)
	if err != nil {
		return nil, ErrUnavailable
	}
	list := []historyEventRow{}
	ids := []string{}
	for rows.Next() {
		var row historyEventRow
		e := &row.event
		if err = rows.Scan(&e.ID, &e.InstanceID, &e.FlowID, &row.sourceView, &row.definition, &e.ActorID, &e.Action, &e.Outcome, &e.Sequence, &e.SchemaVersion, &e.RecordVersion, &row.at, &row.proof, &row.version, &row.task, &row.epoch, &e.NodeID, &e.TargetNodeID, &e.FlowName, &e.FlowNameSource); err != nil {
			rows.Close()
			return nil, ErrUnavailable
		}
		list = append(list, row)
		ids = append(ids, e.ID)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, ErrUnavailable
	}
	entries, err := (fc.Ledger{Namespace: "applications"}).GetManyInTx(ctx, tx, ids)
	if err != nil {
		return nil, ErrUnavailable
	}
	out := make([]WorkflowEventSummary, 0, len(list))
	for _, row := range list {
		e := row.event
		entry := entries[e.ID]
		c, r := entry.Command, entry.Receipt
		if !validWorkflowJournalMetadata(e, c, row.version, row.task, row.epoch) || c.ProtocolVersion != 2 || c.AppID != q.AppID || c.TableID != table || c.RecordID != q.RecordID || c.ViewID != row.sourceView || c.InstanceID != e.InstanceID || c.FlowID != e.FlowID || c.DefinitionVersion != row.definition || c.ActorID != e.ActorID || c.Action != e.Action || c.SchemaVersion != e.SchemaVersion || c.RecordVersion != e.RecordVersion || r == nil || entry.State != e.Outcome || r.Outcome != e.Outcome || r.Sequence != e.Sequence || r.ProofID != row.proof || row.at.IsZero() {
			return nil, ErrUnavailable
		}
		e.OccurredAt = row.at.UTC().Format(time.RFC3339Nano)
		out = append(out, e)
	}
	return out, nil
}
