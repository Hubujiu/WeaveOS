package appstructure

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"time"
)

type HistoryChange struct {
	FieldID   string          `json:"fieldId"`
	FieldKind string          `json:"fieldKind"`
	Before    json.RawMessage `json:"before"`
	After     json.RawMessage `json:"after"`
}
type HistoryEvent struct {
	ID                  string          `json:"id"`
	RecordVersionBefore int64           `json:"recordVersionBefore"`
	RecordVersionAfter  int64           `json:"recordVersionAfter"`
	ActorID             string          `json:"actorId"`
	OccurredAt          time.Time       `json:"occurredAt"`
	Origin              string          `json:"origin"`
	Changes             []HistoryChange `json:"changes"`
}
type HistoryMutation struct {
	AppID, TableID, ViewID, RecordID, ActorID, OperationID string
	RecordVersionBefore, RecordVersionAfter                int64
	Origin                                                 string
	OpaqueTaskRef                                          *string
	OccurredAt                                             time.Time
	Changes                                                []HistoryChange
}
type RecordHistoryWriter interface {
	Append(context.Context, pgx.Tx, HistoryMutation) error
}
type RecordHistoryStore struct{}

type RecordHistoryDML struct {
	History       RecordHistoryWriter
	Origin        string
	OpaqueTaskRef *string
}

func (RecordHistoryDML) LockHeader(c context.Context, tx pgx.Tx, t RecordTable, id string) (StoredRecordHeader, error) {
	return (RecordDML{}).LockHeader(c, tx, t, id)
}

type HistoryPage struct {
	Items   []HistoryEvent `json:"items"`
	HasMore bool           `json:"-"`
}
type HistoryRead struct {
	AppID, TableID, ViewID, RecordID string
	FieldIDs                         []string
	AllFields                        bool
	PageSize                         int
	AfterTime                        *time.Time
	AfterID                          *string
}
