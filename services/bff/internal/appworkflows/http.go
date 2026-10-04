package appworkflows

import (
 "net/http"
 "github.com/Hubujiu/WeaveOS/services/bff/internal/appschema"
 "github.com/Hubujiu/WeaveOS/services/bff/internal/session"
 "github.com/jackc/pgx/v5/pgxpool"
)

type Application struct { Pool *pgxpool.Pool; Limits appschema.Limits }
type Service struct {
 Application *Application
 Authenticator session.Authenticator
 TrustedProxyHosts []string
}
func(*Service) ServeHTTP(w http.ResponseWriter,r *http.Request){http.Error(w,"workflow HTTP not implemented",http.StatusServiceUnavailable)}
