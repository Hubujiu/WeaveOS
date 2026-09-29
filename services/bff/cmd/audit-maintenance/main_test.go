package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMaintenanceRequiresExplicitSingleRun(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "maintenance.exe")
	if output, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %s: %v", output, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary)
	cmd.Env = append(os.Environ(), "WEAVEOS_AUDIT_LIVE_DATABASE_URL="+os.Getenv("WEAVEOS_TEST_DATABASE_URL"), "WEAVEOS_AUDIT_COLD_DATABASE_URL="+os.Getenv("WEAVEOS_TEST_DATABASE_URL"))
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatal("without --once must reject immediately, not enter an internal schedule")
	}
	if err == nil || !strings.Contains(string(output), "audit maintenance failed") {
		t.Fatal("missing --once must fail with redacted diagnostic")
	}
}
