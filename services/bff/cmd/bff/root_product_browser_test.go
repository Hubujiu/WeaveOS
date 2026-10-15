//go:build workflowbrowser_integration && workflowruntime_integration && workflowrpc_integration && linux

package main

import (
	"encoding/json"
	fc "github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// This is a real-process/browser integration fixture, not a browser mock server.
// Frontend assets are the exact build. All API/health requests reach actual BFF.
func TestRootProductBrowserJourney(t *testing.T) {
	browser, head, exchange, web := os.Getenv("WEAVEOS_PRODUCT_BROWSER"), os.Getenv("WEAVEOS_PRODUCT_HEAD"), os.Getenv("WEAVEOS_PRODUCT_EXCHANGE_DIR"), os.Getenv("WEAVEOS_PRODUCT_WEB_ROOT")
	if (browser != "chromium" && browser != "firefox" && browser != "webkit") || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(head) || !filepath.IsAbs(exchange) || !filepath.IsAbs(web) {
		t.Fatal("isolated product fixture environment required")
	}
	if _, err := os.Stat(filepath.Join(web, "index.html")); err != nil {
		t.Fatal("exact built frontend missing")
	}
	x := rootFormalSetupEditableIngress(t, true, func(api http.Handler) http.Handler {
		assets := http.FileServer(http.Dir(web))
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/health/") {
				api.ServeHTTP(w, r)
				return
			}
			if r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/app") || r.URL.Path == "/login" {
				http.ServeFile(w, r, filepath.Join(web, "index.html"))
				return
			}
			assets.ServeHTTP(w, r)
		})
	})
	f := x.f
	otherSession, otherCSRF, err := f.store.Create(f.ctx, session.Record{UserID: f.other, SessionRef: f.id(t), AuthVersion: "1"})
	if err != nil {
		t.Fatal("second isolated Session unavailable")
	}
	t.Cleanup(func() { _, _ = f.store.Revoke(f.ctx, otherSession) })
	fixture := map[string]any{"version": 1, "head": head, "browser": browser, "origin": f.server.URL, "actorId": f.actor, "outsiderId": f.other, "appId": f.app, "viewId": f.view, "recordId": f.record, "instanceId": f.instance, "taskId": f.task, "fieldId": f.field, "session": f.sid, "csrf": f.csrf, "otherSession": otherSession, "otherCsrf": otherCSRF}
	raw, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal("fixture encoding failed")
	}
	private := filepath.Join(exchange, browser+".request.json")
	temporary := private + ".tmp"
	if err = os.WriteFile(temporary, raw, 0600); err != nil {
		t.Fatal("private exchange unavailable")
	}
	t.Cleanup(func() { _ = os.Remove(private); _ = os.Remove(temporary) })
	if err = os.Rename(temporary, private); err != nil {
		t.Fatal("private fixture publication failed")
	}
	done := filepath.Join(exchange, browser+".done.json")
	deadline := time.NewTimer(90 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		raw, e := os.ReadFile(done)
		if e == nil {
			var result struct {
				Browser string `json:"browser"`
				Success bool   `json:"success"`
			}
			if json.Unmarshal(raw, &result) != nil || result.Browser != browser || !result.Success {
				t.Fatal("real browser did not complete")
			}
			break
		}
		if !os.IsNotExist(e) {
			t.Fatal("browser completion unreadable")
		}
		select {
		case <-deadline.C:
			t.Fatal("browser did not complete within fixture bound")
		case <-f.ctx.Done():
			t.Fatal("fixture context expired")
		case <-ticker.C:
		}
	}
	relation := pgx.Identifier{"appdata", "t_" + strings.ReplaceAll(f.table, "-", "")}.Sanitize()
	column := pgx.Identifier{"f_" + strings.ReplaceAll(f.field, "-", "")}.Sanitize()
	var value, state string
	var version, sequence int64
	if err = f.owner.QueryRow(f.ctx, "SELECT "+column+",record_version FROM "+relation+" WHERE id=$1", f.record).Scan(&value, &version); err != nil || value != "browser corrected "+browser || version != 2 {
		t.Fatal("browser Save was not independently persisted", err)
	}
	if err = f.owner.QueryRow(f.ctx, "SELECT state,sequence FROM applications.workflow_instances WHERE id=$1", f.instance).Scan(&state, &sequence); err != nil || state != "completed" || sequence != 2 {
		t.Fatal("browser Agree lacks final projection", err)
	}
	var count int
	for _, q := range []string{
		"SELECT count(*) FROM applications.record_change_events WHERE app_id=$1 AND origin='task_save' AND record_version_before=1 AND record_version_after=2",
		"SELECT count(*) FROM applications.workflow_execution_events WHERE app_id=$1 AND action='agree' AND outcome='success' AND record_version=2",
	} {
		if err = f.owner.QueryRow(f.ctx, q, f.app).Scan(&count); err != nil || count != 1 {
			t.Fatal("browser history is not exactly once", err)
		}
	}
	if err = f.owner.QueryRow(f.ctx, "SELECT count(*) FROM applications.workflow_tasks WHERE app_id=$1 AND instance_id=$2 AND closed_command_id IS NULL", f.app, f.instance).Scan(&count); err != nil || count != 0 {
		t.Fatal("completed workflow retained active tasks", err)
	}
	var commandRaw []byte
	if err = f.owner.QueryRow(f.ctx, "SELECT c.command_json FROM applications.workflow_commands c JOIN applications.workflow_execution_events e ON e.command_id=c.command_id WHERE e.app_id=$1 AND e.instance_id=$2 AND e.action='agree' AND e.outcome='success'", f.app, f.instance).Scan(&commandRaw); err != nil {
		t.Fatal("confirmed command missing", err)
	}
	var command fc.Command
	if json.Unmarshal(commandRaw, &command) != nil || command.ActorID != f.actor || command.RecordVersion != 2 {
		t.Fatal("browser approval did not bind saved version")
	}
	receipt, found, err := x.execution.Lookup(f.ctx, command)
	if err != nil || !found || receipt == nil || receipt.Result.State != "completed" {
		t.Fatal("actual Java engine receipt missing", err)
	}
}
