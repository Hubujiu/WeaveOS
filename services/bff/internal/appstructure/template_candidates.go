package appstructure

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

// TemplateReferenceCandidates authorizes the creation purpose in the same
// read-only snapshot as target discovery. It never borrows source-app access.
func (a *Application) TemplateReferenceCandidates(ctx context.Context, p session.Principal, kind, q string, size int, token string) ([]any, Pagination, error) {
	if a == nil || a.Pool == nil {
		return nil, Pagination{}, ErrUnavailable
	}
	if kind != "member" && kind != "department" || size < 1 || size > 50 || !utf8.ValidString(q) || utf8.RuneCountInString(q) > 100 || strings.ContainsRune(q, 0) {
		return nil, Pagination{}, applications.ErrInvalid
	}
	tx, e := (&applications.Application{Pool: a.Pool}).BeginCreateRead(ctx, p)
	if e != nil {
		return nil, Pagination{}, e
	}
	defer tx.Rollback(context.Background())
	items, page, e := a.candidatePage(ctx, tx, candidateCursor{Actor: p.UserID, Query: q, Kind: kind, TaskScope: "template-import"}, size, token)
	if e == nil {
		e = tx.Commit(ctx)
	}
	return items, page, e
}
