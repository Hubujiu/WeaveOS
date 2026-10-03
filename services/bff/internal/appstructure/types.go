package appstructure

import (
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appschema"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type Application struct {
	Pool              *pgxpool.Pool
	ConfirmationKey   []byte
	ConfirmationKeyID string
	Limits            appschema.Limits
	Dependencies      DependencyRegistry
	References        ReferenceValidator
	Now               func() time.Time
}
type Service struct {
	Application       *Application
	Authenticator     session.Authenticator
	TrustedProxyHosts []string
}
