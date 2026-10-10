package appstructure

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appschema"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
)

func InitializeTemplateTableInTx(context.Context, pgx.Tx, session.Principal, string, string, []appfields.Field, appschema.Limits) error {
	return ErrUnavailable
}
