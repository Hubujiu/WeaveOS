package audit

import (
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
)

type Reader struct {
	Authentication *session.Authenticator
	Live           *pgxpool.Pool
}

func (r *Reader) List(request *http.Request, limit int) ([]json.RawMessage, error) { return nil, nil }
