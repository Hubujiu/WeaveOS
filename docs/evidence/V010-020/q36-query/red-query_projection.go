package personnel

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"time"
)

type ProjectionLabel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type MemberProjection struct {
	ID              string            `json:"id"`
	CreatedAt       time.Time         `json:"createdAt"`
	Account         string            `json:"account"`
	Status          string            `json:"status"`
	BootstrapAdmin  bool              `json:"bootstrapAdmin"`
	PersonnelManage bool              `json:"personnelManage"`
	Departments     []ProjectionLabel `json:"departments"`
	Identities      []ProjectionLabel `json:"identities"`
}
type MemberProjectionPage struct {
	Items       []MemberProjection
	Total       int64
	Fingerprint string
}

func (a *Application) MemberProjection(ctx context.Context, p session.Principal, input MemberQueryInput) (MemberProjectionPage, error) {
	return MemberProjectionPage{}, ErrNotImplemented
}
func scanMemberProjection(ctx context.Context, tx pgx.Tx, input MemberQueryInput) (MemberProjectionPage, error) {
	return MemberProjectionPage{}, ErrNotImplemented
}
