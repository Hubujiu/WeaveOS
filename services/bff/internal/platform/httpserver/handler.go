package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// ReadyCheck verifies the dependencies needed to serve business requests.
// A nil check means composition is incomplete, never an implicit success.
type ReadyCheck func(context.Context) error

func NewHandler(check ReadyCheck, business ...http.Handler) http.Handler {
	var proxies []string
	if len(business) == 1 {
		if source, ok := business[0].(interface{ TrustedProxies() []string }); ok {
			proxies = source.TrustedProxies()
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		r, err = Prepare(w, r, proxies)
		if err != nil {
			http.Error(w, "request initialization failed", http.StatusServiceUnavailable)
			return
		}
		requestID := Metadata(r.Context()).RequestID
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		write := func(status int, body any) {
			w.WriteHeader(status)
			if r.Method != http.MethodHead {
				_ = json.NewEncoder(w).Encode(body)
			}
		}
		if r.URL.Path == "/health/live" || r.URL.Path == "/health/ready" {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.Header().Set("Allow", "GET, HEAD")
				write(http.StatusMethodNotAllowed, map[string]string{"status": "method_not_allowed"})
				return
			}
			if r.URL.Path == "/health/live" {
				write(http.StatusOK, map[string]string{"status": "alive"})
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if check == nil || ctx.Err() != nil || check(ctx) != nil || ctx.Err() != nil {
				write(http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
				return
			}
			write(http.StatusOK, map[string]string{"status": "ready"})
			return
		}
		if len(business) == 1 && business[0] != nil && strings.HasPrefix(r.URL.Path, "/api/") {
			business[0].ServeHTTP(w, r)
			return
		}
		write(http.StatusNotFound, struct {
			Code    string            `json:"code"`
			Message string            `json:"message"`
			Data    any               `json:"data"`
			Meta    map[string]string `json:"meta"`
		}{"API_NOT_FOUND", "Resource not found", nil, map[string]string{"requestId": requestID}})
	})
}
