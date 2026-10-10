package workflowcatalog

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

// DeletionInput is internal; the caller must hold current management authority.
type DeletionInput struct {
	AppID, ViewID, FlowID, ActorID, OperationID string
	ExpectedRevision                            int64
}

type Deletion struct {
	AppID, TableID, ViewID, FlowID, ActorID, OperationID, FlowName, Status string
	ExpectedRevision                                                       int64
	Reason                                                                 *string
	DeletedVersions                                                        *int64
	DeletedAt, CompletedAt                                                 *time.Time
}

// RequestDeletionInTx closes new work and captures the original authorized intent.
// The caller must commit the transaction before reporting acceptance.
func (Catalog) RequestDeletionInTx(ctx context.Context, tx pgx.Tx, in DeletionInput) (Deletion, error) {
	if tx == nil || !validIDs(in.AppID, in.ViewID, in.FlowID, in.ActorID, in.OperationID) || in.ExpectedRevision < 1 || in.ExpectedRevision >= maxSafeInteger {
		return Deletion{}, ErrInvalid
	}
	for _, id := range []string{in.AppID, in.ViewID, in.FlowID, in.ActorID, in.OperationID} {
		if id == "00000000-0000-0000-0000-000000000000" {
			return Deletion{}, ErrInvalid
		}
	}
	raw, _ := json.Marshal(in)
	fingerprint := sha256.Sum256(raw)
	// Serialize acceptance with existing application mutations. Replay never
	// requires the definition/version to survive subsequent cleanup.
	var app string
	if e := tx.QueryRow(ctx, `SELECT id::text FROM applications.apps WHERE id=$1 FOR UPDATE`, in.AppID).Scan(&app); e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			return Deletion{}, ErrMissing
		}
		return Deletion{}, mapDBError(e)
	}
	var d Deletion
	var original []byte
	e := tx.QueryRow(ctx, `SELECT `+deletionColumns+`,fingerprint FROM applications.workflow_deletions WHERE flow_id=$1 OR (actor_user_id=$2 AND operation_id=$3)`, in.FlowID, in.ActorID, in.OperationID).Scan(deletionTargets(&d, &original)...)
	if e == nil {
		if d.AppID != in.AppID || d.ViewID != in.ViewID || d.FlowID != in.FlowID || d.ActorID != in.ActorID || d.OperationID != in.OperationID || !bytes.Equal(original, fingerprint[:]) {
			return Deletion{}, ErrConflict
		}
		return d, nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return Deletion{}, mapDBError(e)
	}
	h, _, e := lockFlow(ctx, tx, in.AppID, in.FlowID, false)
	if e != nil {
		return Deletion{}, e
	}
	if h.ViewID != in.ViewID {
		return Deletion{}, ErrMissing
	}
	if h.Revision != in.ExpectedRevision {
		return Deletion{}, ErrConflict
	}
	_, e = tx.Exec(ctx, `UPDATE applications.workflow_definitions SET state='closing',revision=revision+1,close_epoch=close_epoch+1,updated_at=clock_timestamp() WHERE app_id=$1 AND id=$2`, in.AppID, in.FlowID)
	if e != nil {
		return Deletion{}, mapDBError(e)
	}
	e = tx.QueryRow(ctx, `INSERT INTO applications.workflow_deletions(flow_id,app_id,table_id,view_id,actor_user_id,operation_id,flow_name,expected_revision,fingerprint)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+deletionColumns+`,fingerprint`, in.FlowID, in.AppID, h.TableID, in.ViewID, in.ActorID, in.OperationID, h.Name, in.ExpectedRevision, fingerprint[:]).Scan(deletionTargets(&d, &original)...)
	return d, mapDBError(e)
}

const deletionColumns = `app_id::text,table_id::text,view_id::text,flow_id::text,actor_user_id::text,operation_id::text,flow_name,status,expected_revision,reason,engine_deleted_versions,engine_deleted_at,completed_at`

func deletionTargets(d *Deletion, fingerprint *[]byte) []any {
	return []any{&d.AppID, &d.TableID, &d.ViewID, &d.FlowID, &d.ActorID, &d.OperationID, &d.FlowName, &d.Status, &d.ExpectedRevision, &d.Reason, &d.DeletedVersions, &d.DeletedAt, &d.CompletedAt, fingerprint}
}
func rejectDeletion(ctx context.Context, tx pgx.Tx, flowID string) error {
	var deleting bool
	if e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM applications.workflow_deletions WHERE flow_id=$1)`, flowID).Scan(&deleting); e != nil {
		return mapDBError(e)
	}
	if deleting {
		return ErrClosing
	}
	return nil
}
