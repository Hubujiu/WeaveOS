package personnel

import (
	"context"
	"github.com/jackc/pgx/v5"
)

// Compile-only RED declarations. The query engine will call the page reader
// inside the same authorized RR after validating its current revision vector.
func memberPageQuery(input MemberQueryInput) (string, []any, PageQuery, error) {
	return "", nil, PageQuery{}, ErrNotImplemented
}
func scanMemberPage(ctx context.Context, tx pgx.Tx, input MemberQueryInput) ([]MemberProjection, error) {
	return nil, ErrNotImplemented
}
