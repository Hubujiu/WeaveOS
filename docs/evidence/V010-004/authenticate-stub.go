package session

import (
 "context"
 "errors"
 "net/http"
 "github.com/Hubujiu/WeaveOS/services/bff/internal/persistence/authsql"
)

var ErrUnauthorized=errors.New("unauthenticated")
var ErrForbidden=errors.New("request forbidden")
var ErrUnavailable=errors.New("authentication dependency unavailable")

type Authenticator struct { Sessions *Store; DB authsql.DBTX; Origin string }
type Principal struct { UserID,Account,SessionRef string; BootstrapAdmin bool; Record Record; SID string }
func (a *Authenticator) Authenticate(r *http.Request,write bool)(Principal,error){return Principal{},ErrUnauthorized}
func (a *Authenticator) Renew(ctx context.Context,w http.ResponseWriter,r *http.Request,p Principal)error{return ErrUnauthorized}
