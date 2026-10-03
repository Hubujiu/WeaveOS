package appstructure

import (
  "encoding/json"
  "github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
  "github.com/Hubujiu/WeaveOS/services/bff/internal/session"
  "net/http/httptest"
  "os"
  "testing"
  "time"
)

// Temporary detached-worktree bridge: one real HTTP service, PG18, Redis session.
func TestV014BrowserBridge(t *testing.T) {
  f := setup(t)
  f.service.Application.References = CurrentSources{}
  appService := &applications.Service{Application:&applications.Application{Pool:f.runtime},
    Authenticator:session.Authenticator{Sessions:f.session,DB:f.runtime,Origin:"https://weaveos.test"},
    Definitions:f.service}
  server := httptest.NewServer(appService)
  defer server.Close()
  bridge := map[string]string{"url": server.URL, "appId": f.app, "actorId": f.actor, "sid": f.sid, "csrf": f.csrf}
  raw, err := json.Marshal(bridge); if err != nil { t.Fatal(err) }
  if err := os.WriteFile("/tmp/v014-browser-bridge.json", raw, 0600); err != nil { t.Fatal(err) }
  t.Logf("V014_BRIDGE_READY %s", server.URL)
  deadline := time.After(15 * time.Minute)
  ticker := time.NewTicker(200 * time.Millisecond); defer ticker.Stop()
  for { select {
    case <-deadline: return
    case <-ticker.C: if _, err := os.Stat("/tmp/v014-browser-bridge.stop"); err == nil { return }
  }}
}
