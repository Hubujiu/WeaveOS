package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMaintenanceRequiresExplicitSingleRun(t *testing.T) {
	live, cold := maintenanceDatabases(t)
	binary := filepath.Join(t.TempDir(), "maintenance.exe")
	if output, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %s: %v", output, err)
	}
	env := []string{}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "WEAVEOS_AUDIT_LIVE_DATABASE_URL=") && !strings.HasPrefix(entry, "WEAVEOS_AUDIT_COLD_DATABASE_URL=") {
			env = append(env, entry)
		}
	}
	env = append(env, "WEAVEOS_AUDIT_LIVE_DATABASE_URL="+live, "WEAVEOS_AUDIT_COLD_DATABASE_URL="+cold)
	run := func(timeout time.Duration, args ...string) ([]byte, error) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Env = env
		output, err := cmd.CombinedOutput()
		if ctx.Err() != nil {
			t.Fatalf("maintenance did not finish within test deadline: args=%v", args)
		}
		return output, err
	}
	// Prove the exact same fixture is valid before removing only the required flag.
	output, err := run(10*time.Second, "--once")
	var message struct {
		Level   string `json:"level"`
		Message string `json:"msg"`
	}
	if err != nil {
		t.Fatalf("valid --once invocation must succeed: %s: %v", output, err)
	}
	if err := json.Unmarshal(output, &message); err != nil || message.Level != "INFO" || message.Message != "audit maintenance complete" {
		t.Fatalf("expected successful single-run receipt: %s (%v)", output, err)
	}
	output, err = run(3 * time.Second)
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		t.Fatalf("missing --once must exit 1: %s: %v", output, err)
	}
	if err := json.Unmarshal(output, &message); err != nil || message.Level != "ERROR" || message.Message != "audit maintenance failed" {
		t.Fatalf("expected exact redacted rejection: %s (%v)", output, err)
	}
}
