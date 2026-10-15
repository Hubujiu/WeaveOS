package appstructure

import (
	"context"
	"encoding/hex"
	"strings"
	"unicode/utf8"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/jackc/pgx/v5"
)

// TrustedTaskCandidateScope is constructed only after the task service checks
// the current assignment and full field permission intersection in this tx.
// Binding is a digest, never the raw SessionRef or business values.
type TrustedTaskCandidateScope struct {
	ActorID, AppID, ViewID, RecordID, FieldID, Kind, Binding string
	PolicyRevision                                           int64
}

// TaskReferenceCandidatesInTx reuses the source SQL and bounded Redis cursor.
// It is an internal transaction port, not an HTTP authorization alternative.
func (a *Application) TaskReferenceCandidatesInTx(ctx context.Context, tx pgx.Tx, scope TrustedTaskCandidateScope, q string, size int, token string) ([]any, Pagination, error) {
	if a == nil || tx == nil {
		return nil, Pagination{}, ErrUnavailable
	}
	for _, id := range []string{scope.ActorID, scope.AppID, scope.ViewID, scope.RecordID, scope.FieldID} {
		if !appfields.ValidID(id) || id == "00000000-0000-0000-0000-000000000000" {
			return nil, Pagination{}, invalid()
		}
	}
	digest, err := hex.DecodeString(scope.Binding)
	if err != nil || len(digest) != 32 || hex.EncodeToString(digest) != scope.Binding || scope.PolicyRevision < 1 || scope.Kind != "member" && scope.Kind != "department" || size < 1 || size > 50 || !utf8.ValidString(q) || utf8.RuneCountInString(q) > 100 || strings.ContainsRune(q, 0) {
		return nil, Pagination{}, invalid()
	}
	cursor := candidateCursor{Actor: scope.ActorID, App: scope.AppID, View: scope.ViewID, Record: scope.RecordID, Field: scope.FieldID, Kind: scope.Kind, Action: "task", Query: q, PolicyRevision: scope.PolicyRevision, TaskScope: scope.Binding}
	return a.candidatePage(ctx, tx, cursor, size, token)
}
