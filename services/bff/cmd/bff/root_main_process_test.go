package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type rootBFFProcess struct {
	command *exec.Cmd
	done    chan struct{}
	err     error
	output  string
}

func rootMainEnvironment(cfg config, addr string) map[string]string {
	return map[string]string{"BFF_ADDR": addr, "WEAVEOS_DATABASE_URL": cfg.DatabaseURL, "WEAVEOS_REDIS_URL": cfg.RedisURL, "WEAVEOS_PUBLIC_ORIGIN": cfg.Origin, "WEAVEOS_SESSION_GENERATION": cfg.Generation, "WEAVEOS_AUDIT_KEY_ID": cfg.AuditKeyID, "WEAVEOS_AUDIT_HMAC_KEY": base64.StdEncoding.EncodeToString(cfg.AuditKey), "WEAVEOS_DEFINITION_KEY_ID": cfg.DefinitionKeyID, "WEAVEOS_DEFINITION_HMAC_KEY": base64.StdEncoding.EncodeToString(cfg.DefinitionKey), "WEAVEOS_SCHEMA_LOCK_TIMEOUT_MS": "1000", "WEAVEOS_SCHEMA_STATEMENT_TIMEOUT_MS": "5000", "WEAVEOS_WORKFLOW_ENABLED": "true", "WEAVEOS_WORKFLOW_TARGET": cfg.WorkflowRuntime.Target, "WEAVEOS_WORKFLOW_RPC_TIMEOUT_MS": "1000", "WEAVEOS_WORKFLOW_SERVICE_TOKEN": rootRPCToken}
}
func rootMainLaunch(t *testing.T, env map[string]string) *rootBFFProcess {
	t.Helper()
	dir := t.TempDir()
	binary := filepath.Join(dir, "bff")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, e := build.CombinedOutput(); e != nil {
		t.Fatalf("actual BFF binary build failed: %v %s", e, output)
	}
	path := filepath.Join(dir, "bff.log")
	file, e := os.Create(path)
	if e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(binary)
	cmd.Env = []string{"LANG=C.UTF-8"}
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Stdout = file
	cmd.Stderr = file
	if e = cmd.Start(); e != nil {
		_ = file.Close()
		t.Fatal(e)
	}
	p := &rootBFFProcess{command: cmd, done: make(chan struct{}), output: path}
	go func() { p.err = cmd.Wait(); _ = file.Close(); close(p.done) }()
	t.Cleanup(func() {
		select {
		case <-p.done:
		default:
			_ = cmd.Process.Kill()
			select {
			case <-p.done:
			case <-time.After(5 * time.Second):
				t.Error("owned subprocess failed to exit")
			}
		}
		if t.Failed() {
			raw, _ := os.ReadFile(path)
			safe := string(raw)
			for k, v := range env {
				if strings.Contains(k, "KEY") || strings.Contains(k, "TOKEN") || strings.Contains(k, "URL") {
					safe = strings.ReplaceAll(safe, v, "[redacted fixture]")
				}
			}
			t.Log(safe)
		}
	})
	return p
}
func (p *rootBFFProcess) wait(t *testing.T) error {
	t.Helper()
	select {
	case <-p.done:
		return p.err
	case <-time.After(15 * time.Second):
		t.Fatal("actual BFF process did not exit within bound")
		return nil
	}
}
func rootMainAddress(t *testing.T) string {
	t.Helper()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}
func rootMainReady(t *testing.T, p *rootBFFProcess, addr string) {
	t.Helper()
	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-p.done:
			t.Fatal("formal BFF exited before readiness")
		default:
		}
		r, e := client.Get("http://" + addr + "/health/ready")
		if e == nil {
			_, _ = io.Copy(io.Discard, r.Body)
			_ = r.Body.Close()
			if r.StatusCode == 200 {
				return
			}
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatal("formal BFF did not become ready")
}
func rootMainSafeOutput(t *testing.T, p *rootBFFProcess, env map[string]string) {
	t.Helper()
	b, e := os.ReadFile(p.output)
	if e != nil {
		t.Fatal(e)
	}
	for _, key := range []string{"WEAVEOS_DATABASE_URL", "WEAVEOS_REDIS_URL", "WEAVEOS_WORKFLOW_SERVICE_TOKEN", "WEAVEOS_AUDIT_HMAC_KEY", "WEAVEOS_DEFINITION_HMAC_KEY"} {
		if v := env[key]; v != "" && strings.Contains(string(b), v) {
			t.Fatal("formal process log contains a credential/configuration value")
		}
	}
}

func TestRootMainProcessSigtermStopsClaimsBeforeWaitingForActiveHttp(t *testing.T) {
	f := rootHTTPTaskSetup(t)
	runtime := rootHostSetup(t)
	addr := rootMainAddress(t)
	operation := f.id(t)
	var bpmn string
	var epoch, revision int64
	if e := f.owner.QueryRow(f.ctx, "SELECT v.bpmn_xml,d.close_epoch,d.revision FROM applications.workflow_definitions d JOIN applications.workflow_versions v ON v.app_id=d.app_id AND v.flow_id=d.id AND v.version=1 WHERE d.app_id=$1 AND d.id=$2", f.app, f.flow).Scan(&bpmn, &epoch, &revision); e != nil {
		t.Fatal(e)
	}
	hash := sha256.Sum256([]byte(bpmn))
	fingerprint := sha256.Sum256([]byte("independent accepted-before-shutdown intent"))
	_, e := f.owner.Exec(f.ctx, "INSERT INTO applications.workflow_publications(actor_user_id,operation_id,app_id,view_id,flow_id,version,version_id,bpmn_sha256,actor_auth_version,accepted_close_epoch,expected_revision,expected_schema_version,fingerprint,next_attempt_at) VALUES($1,$2,$3,$4,$5,1,$6,$7,1,$8,$9,1,$10,'9999-12-31T00:00:00Z')", f.actor, operation, f.app, f.view, f.flow, f.version, hash[:], epoch, revision, fingerprint[:])
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, e := f.owner.Exec(ctx, "DELETE FROM applications.workflow_publications WHERE actor_user_id=$1 AND operation_id=$2 AND status IN ('pending','unknown')", f.actor, operation)
		if e != nil {
			t.Error(e)
		}
	})
	env := rootMainEnvironment(runtime.cfg, addr)
	p := rootMainLaunch(t, env)
	rootMainReady(t, p, addr)
	lock, e := f.owner.Begin(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer lock.Rollback(context.Background())
	if _, e = lock.Exec(f.ctx, "LOCK TABLE auth.users IN ACCESS EXCLUSIVE MODE"); e != nil {
		t.Fatal(e)
	}
	request, e := http.NewRequest("POST", "http://"+addr+"/api/v1/sessions", strings.NewReader(`{"account":"unknown-shutdown-account","password":"Aa1!"}`))
	if e != nil {
		t.Fatal(e)
	}
	request.Header.Set("Origin", runtime.cfg.Origin)
	request.Header.Set("Content-Type", "application/json")
	response := make(chan error, 1)
	go func() {
		r, e := (&http.Client{Timeout: 15 * time.Second}).Do(request)
		if e == nil {
			var raw []byte
			raw, e = io.ReadAll(r.Body)
			_ = r.Body.Close()
			if e == nil {
				var envelope struct {
					Code string `json:"code"`
				}
				if e = json.Unmarshal(raw, &envelope); e == nil && (r.StatusCode != 401 || envelope.Code != "AUTH_INVALID_CREDENTIALS") {
					e = fmt.Errorf("drained login must complete as 401 AUTH_INVALID_CREDENTIALS; got %d %s", r.StatusCode, envelope.Code)
				}
			}
		}
		response <- e
	}()
	deadline := time.Now().Add(5 * time.Second)
	blocked := false
	for time.Now().Before(deadline) {
		var n int
		e = runtime.admin.QueryRow(f.ctx, "SELECT count(*) FROM pg_stat_activity WHERE application_name=$1 AND wait_event_type='Lock'", runtime.applicationName).Scan(&n)
		if e != nil {
			t.Fatal(e)
		}
		if n > 0 {
			blocked = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !blocked {
		t.Fatal("must observe real in-flight HTTP database wait before SIGTERM")
	}
	if e = p.command.Process.Signal(syscall.SIGTERM); e != nil {
		t.Fatal(e)
	}
	deadline = time.Now().Add(3 * time.Second)
	stopped := false
	for time.Now().Before(deadline) {
		c, e := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if e != nil {
			stopped = true
			break
		}
		_ = c.Close()
		time.Sleep(10 * time.Millisecond)
	}
	if !stopped {
		t.Fatal("SIGTERM did not stop HTTP admission")
	}
	if _, e = f.owner.Exec(f.ctx, "UPDATE applications.workflow_publications SET next_attempt_at=clock_timestamp() WHERE actor_user_id=$1 AND operation_id=$2", f.actor, operation); e != nil {
		t.Fatal(e)
	}
	// Existing publication loop idles200ms; retain two seconds of the open HTTP
	// drain to observe whether it incorrectly continues claiming after SIGTERM.
	select {
	case <-p.done:
		t.Fatal("process exited without draining the blocked HTTP request")
	case <-time.After(2 * time.Second):
	}
	var attempts int
	var state string
	if e = f.owner.QueryRow(f.ctx, "SELECT attempts,status FROM applications.workflow_publications WHERE actor_user_id=$1 AND operation_id=$2", f.actor, operation).Scan(&attempts, &state); e != nil {
		t.Fatal(e)
	}
	if e = lock.Rollback(f.ctx); e != nil {
		t.Fatal(e)
	}
	select {
	case err := <-response:
		if err != nil {
			t.Fatalf("accepted in-flight HTTP request did not complete its expected response: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("held HTTP request failed to finish after unlock")
	}
	if e = p.wait(t); e != nil {
		t.Fatal("normal SIGTERM returned failure", e)
	}
	runtime.noConnections(t)
	rootMainSafeOutput(t, p, env)
	if attempts != 0 || state != "pending" {
		t.Fatalf("worker continued after stop while HTTP drained: attempts=%d state=%s", attempts, state)
	}
}
func TestRootMainProcessInvalidIdentityExitsWithoutSecrets(t *testing.T) {
	f := rootHostSetup(t)
	env := rootMainEnvironment(f.cfg, rootMainAddress(t))
	delete(env, "WEAVEOS_WORKFLOW_SERVICE_TOKEN")
	p := rootMainLaunch(t, env)
	if p.wait(t) == nil {
		t.Fatal("incomplete enabled identity exited successfully")
	}
	rootMainSafeOutput(t, p, env)
	f.noConnections(t)
}
func TestRootMainProcessOccupiedAddressExitsAndReleasesInitializedDependencies(t *testing.T) {
	f := rootHostSetup(t)
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	env := rootMainEnvironment(f.cfg, listener.Addr().String())
	p := rootMainLaunch(t, env)
	if p.wait(t) == nil {
		t.Fatal("occupied address silently succeeded")
	}
	rootMainSafeOutput(t, p, env)
	f.noConnections(t)
	f.engine.mu.Lock()
	checks := len(f.engine.services)
	f.engine.mu.Unlock()
	if checks < 2 {
		t.Fatal("test never reached binding after initialized workflow dependencies: " + strconv.Itoa(checks))
	}
}
