package apprecordservice

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/querycontext"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
)

var ErrQueryBusy = errors.New("record query changed during write prevalidation")

// The RR receipt is an interaction guard. It never supplies authorization to
// the RC business transaction, which reacquires live grants and the table gate.
func (s *Service) beginRecordMutation(ctx context.Context, principal session.Principal, appID, viewID, version string, options applications.RecordWriteOptions) (*applications.RecordWrite, *applications.Result, error) {
	for attempt := 0; attempt < 3; attempt++ {
		var verified revisions
		if version != "" {
			strategy := &recordStrategy{service: s, principal: principal, appID: appID, viewID: viewID}
			read, receipt, err := querycontext.ValidateSavedRead(ctx, s.Queries, principal.SessionRef, version, strategy)
			if err != nil {
				// A confirmed original key remains recoverable even if its old
				// list context has subsequently changed or expired.
				write, beginErr := (&applications.Application{Pool: s.Pool}).BeginRecordWrite(ctx, principal, appID, viewID, options)
				if beginErr != nil {
					return nil, nil, beginErr
				}
				replayed, replayErr := write.Replay(ctx, options.OperationID, options.Kind, options.Fingerprint)
				if replayErr != nil {
					_ = write.Rollback(context.Background())
					return nil, nil, replayErr
				}
				if replayed != nil {
					return write, replayed, nil
				}
				_ = write.Rollback(context.Background())
				return nil, nil, err
			}
			if err = json.Unmarshal(receipt.Revision(), &verified); err != nil || !validRevisions(verified) {
				_ = read.Rollback(context.Background())
				return nil, nil, querycontext.ErrInvalid
			}
			if err = receipt.Commit(ctx, read); err != nil {
				return nil, nil, err
			}
		}
		write, err := (&applications.Application{Pool: s.Pool}).BeginRecordWrite(ctx, principal, appID, viewID, options)
		if err != nil {
			return nil, nil, err
		}
		replayed, err := write.Replay(ctx, options.OperationID, options.Kind, options.Fingerprint)
		if err != nil {
			_ = write.Rollback(context.Background())
			return nil, nil, err
		}
		if replayed != nil || version == "" {
			return write, replayed, nil
		}
		facts := write.Context()
		if err = (appstructure.RecordGate{AppID: appID, TableID: facts.TableID, ViewID: viewID}).LockTable(ctx, write.Tx(), facts.TableID, facts.SchemaVersion); err != nil {
			_ = write.Rollback(context.Background())
			return nil, nil, err
		}
		current, err := readWriteRevisions(ctx, write.Tx(), appID, viewID, facts.TableID)
		if err != nil {
			_ = write.Rollback(context.Background())
			return nil, nil, err
		}
		if current != verified {
			_ = write.Rollback(context.Background())
			continue
		}
		return write, nil, nil
	}
	return nil, nil, ErrQueryBusy
}

func readWriteRevisions(ctx context.Context, tx pgx.Tx, appID, viewID, tableID string) (revisions, error) {
	var r revisions
	err := tx.QueryRow(ctx, `SELECT t.data_revision,t.dependency_revision,t.schema_version,v.view_version,a.policy_revision,s.revision
 FROM applications.apps a
 JOIN applications.form_views v ON v.app_id=a.id AND v.id=$2
 JOIN applications.logical_tables t ON t.app_id=a.id AND t.id=v.table_id AND t.id=$3
 CROSS JOIN applications.reference_source_revision s
 WHERE a.id=$1 AND s.singleton`, appID, viewID, tableID).Scan(&r.Data, &r.Dependency, &r.Schema, &r.View, &r.Policy, &r.Source)
	return r, err
}
