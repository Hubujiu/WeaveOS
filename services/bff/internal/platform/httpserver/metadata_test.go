package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
)

type ingressHandler struct {
	http.Handler
	hosts []string
}

func (h ingressHandler) TrustedProxies() []string { return h.hosts }

// FR-02/D3/D4: only the immediate trusted peer can provide ingress metadata.
func TestIngressMetadataTrustAndContext(t *testing.T) {
	const ingressID = "0123456789abcdef0123456789abcdef"
	for _, tc := range []struct {
		name, peer string
		ids, xff   []string
		accepted   bool
		ip         string
	}{
		{"trusted", "127.0.0.1:4321", []string{ingressID}, []string{"198.51.100.9"}, true, "198.51.100.9"},
		{"direct", "192.0.2.7:4321", []string{ingressID}, []string{"198.51.100.9"}, false, "192.0.2.7"},
		{"duplicate-id", "127.0.0.1:4321", []string{ingressID, ingressID}, []string{"198.51.100.9"}, false, "198.51.100.9"},
		{"invalid-id", "127.0.0.1:4321", []string{"not-hex"}, []string{"198.51.100.9"}, false, "198.51.100.9"},
		{"xff-chain", "127.0.0.1:4321", []string{ingressID}, []string{"198.51.100.9, 203.0.113.1"}, true, "127.0.0.1"},
		{"duplicate-xff", "127.0.0.1:4321", []string{ingressID}, []string{"198.51.100.9", "203.0.113.1"}, true, "127.0.0.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var meta httpserver.RequestMetadata
			business := ingressHandler{http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				meta = httpserver.Metadata(r.Context())
				// Reading a copy or modifying HTTP headers must not mutate context facts.
				copy := httpserver.Metadata(r.Context())
				copy.RequestID = "forged-copy"
				r.Header.Set("X-Request-Id", "forged-header")
				if httpserver.Metadata(r.Context()).RequestID != meta.RequestID {
					t.Error("metadata is mutable")
				}
				w.WriteHeader(204)
			}), []string{"127.0.0.1"}}
			r := httptest.NewRequest("GET", "/api/v1/test", nil)
			r.RemoteAddr = tc.peer
			r.Header["X-Request-Id"] = tc.ids
			r.Header["X-Forwarded-For"] = tc.xff
			r.Header.Set("User-Agent", "independent-browser")
			w := httptest.NewRecorder()
			httpserver.NewHandler(nil, business).ServeHTTP(w, r)
			id := w.Header().Get("X-Request-Id")
			if tc.accepted && id != ingressID {
				t.Errorf("trusted ingress id was replaced: %q", id)
			}
			if !tc.accepted && (id == ingressID || !regexp.MustCompile(`^[a-f0-9]{32}$`).MatchString(id)) {
				t.Error("untrusted id was accepted or fallback missing")
			}
			if meta.RequestID != id || meta.ClientIP != tc.ip || meta.UserAgent != "independent-browser" {
				t.Errorf("context metadata mismatch: %+v", meta)
			}
		})
	}
}
