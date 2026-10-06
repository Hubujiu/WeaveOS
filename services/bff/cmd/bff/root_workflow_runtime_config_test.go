package main

import (
	"context"
	"testing"
	"time"
)

func wfGet(values map[string]string) func(string) string {
	return func(k string) string { return values[k] }
}
func TestRootWorkflowConfigDisabledByDefault(t *testing.T) {
	c, e := readWorkflowRuntimeConfig(wfGet(nil))
	if e != nil || c.Enabled || c.Target != "" || c.RPCTimeout != 0 {
		t.Fatalf("disabled result=%+v err=%v", c, e)
	}
}
func TestRootWorkflowConfigRequiresExplicitCompleteEnable(t *testing.T) {
	for _, v := range []map[string]string{{"WEAVEOS_WORKFLOW_ENABLED": "yes"}, {"WEAVEOS_WORKFLOW_ENABLED": "1"}, {"WEAVEOS_WORKFLOW_ENABLED": "TRUE"}, {"WEAVEOS_WORKFLOW_ENABLED": "true"}, {"WEAVEOS_WORKFLOW_TARGET": "flowable:50051"}, {"WEAVEOS_WORKFLOW_ENABLED": "false", "WEAVEOS_WORKFLOW_RPC_TIMEOUT_MS": "5000"}} {
		if _, e := readWorkflowRuntimeConfig(wfGet(v)); e == nil {
			t.Errorf("invalid config accepted: %v", v)
		}
	}
}
func TestRootWorkflowConfigParsesPlaintextInternalTargets(t *testing.T) {
	for _, target := range []string{"flowable:50051", "workflow-engine.internal:9090", "127.0.0.1:50051", "10.0.0.2:50051", "[::1]:50051", "[fd00::2]:50051"} {
		c, e := readWorkflowRuntimeConfig(wfGet(map[string]string{"WEAVEOS_WORKFLOW_ENABLED": "true", "WEAVEOS_WORKFLOW_SERVICE_TOKEN": "V041_SYNTHETIC_TEST_ONLY_1234567890", "WEAVEOS_WORKFLOW_TARGET": target}))
		if e != nil || !c.Enabled || c.Target != target || c.RPCTimeout != 5*time.Second {
			t.Errorf("target %q c=%+v e=%v", target, c, e)
		}
	}
}
func TestRootWorkflowConfigRejectsUnsafeTargetSyntax(t *testing.T) {
	for _, target := range []string{"http://flowable:50051", "dns:///flowable:50051", "user:secret@flowable:50051", "flowable:50051/path", "flowable:50051?q=1", "flowable:50051#x", ":50051", "flowable:0", "flowable:65536", "flowable:abc", "flowable", "0.0.0.0:50051", "[::]:50051", "8.8.8.8:50051", "224.0.0.1:50051", "bad host:50051", "-bad:50051"} {
		if _, e := readWorkflowRuntimeConfig(wfGet(map[string]string{"WEAVEOS_WORKFLOW_ENABLED": "true", "WEAVEOS_WORKFLOW_SERVICE_TOKEN": "V041_SYNTHETIC_TEST_ONLY_1234567890", "WEAVEOS_WORKFLOW_TARGET": target})); e == nil {
			t.Errorf("bad target accepted %q", target)
		}
	}
}
func TestRootWorkflowConfigBoundsDeadline(t *testing.T) {
	for _, ms := range []string{"0", "-1", "30001", "1s", "1.5", "99999999999999999999999"} {
		if _, e := readWorkflowRuntimeConfig(wfGet(map[string]string{"WEAVEOS_WORKFLOW_ENABLED": "true", "WEAVEOS_WORKFLOW_SERVICE_TOKEN": "V041_SYNTHETIC_TEST_ONLY_1234567890", "WEAVEOS_WORKFLOW_TARGET": "flowable:50051", "WEAVEOS_WORKFLOW_RPC_TIMEOUT_MS": ms})); e == nil {
			t.Errorf("bad timeout accepted %q", ms)
		}
	}
	for raw, want := range map[string]time.Duration{"1": time.Millisecond, "30000": 30 * time.Second} {
		c, e := readWorkflowRuntimeConfig(wfGet(map[string]string{"WEAVEOS_WORKFLOW_ENABLED": "true", "WEAVEOS_WORKFLOW_SERVICE_TOKEN": "V041_SYNTHETIC_TEST_ONLY_1234567890", "WEAVEOS_WORKFLOW_TARGET": "flowable:50051", "WEAVEOS_WORKFLOW_RPC_TIMEOUT_MS": raw}))
		if e != nil || c.RPCTimeout != want {
			t.Errorf("deadline %s: %+v %v", raw, c, e)
		}
	}
}
func TestRootReadConfigWiresWorkflowValidation(t *testing.T) {
	if _, e := readConfig(wfGet(map[string]string{"WEAVEOS_WORKFLOW_ENABLED": "true"})); e == nil {
		t.Fatal("readConfig ignored incomplete workflow configuration")
	}
}
func TestRootWorkflowEnabledCannotSilentlyUseUnconfiguredHandler(t *testing.T) {
	c, e := readConfig(wfGet(map[string]string{"WEAVEOS_WORKFLOW_ENABLED": "true", "WEAVEOS_WORKFLOW_SERVICE_TOKEN": "V041_SYNTHETIC_TEST_ONLY_1234567890", "WEAVEOS_WORKFLOW_TARGET": "flowable:50051"}))
	if e != nil {
		t.Fatal(e)
	}
	_, close, e := buildHandler(context.Background(), c)
	if close != nil {
		close()
	}
	if e == nil {
		t.Fatal("explicit workflow enable silently ignored")
	}
}
