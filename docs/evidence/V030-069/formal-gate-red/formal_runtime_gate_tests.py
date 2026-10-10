"""Root-owned exact formal-process evidence contract, independent case names."""
import copy
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("formal_gate", Path(__file__).with_name("formal_runtime_gate.py"))
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)
PACKAGE = "github.com/Hubujiu/WeaveOS/services/bff/cmd/bff"
CASES = ["TestRootFormalRuntimePublishAndApprove", "TestRootFormalRuntimeLostReplyBffRestart", "TestRootFormalRuntimeEngineRestart", "TestRootFormalRuntimeNodeSaveThenApproveLatest", "TestRootFormalRuntimeReturnThenWithdrawRecovery", "TestRootFormalRuntimeCloseWaitsForConfirmedProjection", "TestRootFormalRuntimeChildFailureDiagnosticsSurviveCleanup", "TestRootFormalRuntimeDeletionDrainsAndRecoversOriginalReceipt", "TestRootFormalRuntimeRoundResubmitReviewAndRecovery", "TestRootFormalRuntimeRoundIsolation"]
EXPECTED = CASES + [CASES[0] + "/agree", CASES[0] + "/reject"]

class RootFormalRuntimeGateTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.path = Path(self.temp.name) / "formal.jsonl"
        self.events = [{"Action": "start", "Package": PACKAGE}]
        for name in CASES:
            self.events.append({"Action": "run", "Package": PACKAGE, "Test": name})
            if name == CASES[0]:
                for action in ("agree", "reject"):
                    child = name + "/" + action
                    self.events += [{"Action": "run", "Package": PACKAGE, "Test": child}, {"Action": "pass", "Package": PACKAGE, "Test": child}]
            self.events.append({"Action": "pass", "Package": PACKAGE, "Test": name})
        self.events.append({"Action": "pass", "Package": PACKAGE})
    def check(self):
        self.path.write_text("".join(json.dumps(event) + "\n" for event in self.events))
        return gate.validate(self.path)
    def test_exact_scenarios_and_action_variants(self):
        self.assertEqual(12, self.check())
    def test_each_missing_case_rejected(self):
        original = copy.deepcopy(self.events)
        for name in EXPECTED:
            self.events = [event for event in original if event.get("Test") != name]
            with self.assertRaises(gate.GateError): self.check()
    def test_failure_and_skip_rejected(self):
        original = copy.deepcopy(self.events)
        for name in EXPECTED:
            for outcome in ("fail", "skip"):
                self.events = copy.deepcopy(original)
                for event in self.events:
                    if event.get("Test") == name and event["Action"] == "pass": event["Action"] = outcome
                with self.assertRaises(gate.GateError): self.check()
    def test_wrong_package_rejected(self):
        for event in self.events: event["Package"] = "not/the/formal/package"
        with self.assertRaises(gate.GateError): self.check()
    def test_duplicate_case_rejected(self):
        self.events.insert(-1, {"Action": "pass", "Package": PACKAGE, "Test": EXPECTED[0]})
        with self.assertRaises(gate.GateError): self.check()
    def test_unexpected_subcase_rejected(self):
        name = CASES[0] + "/unexpected"
        self.events[-1:-1] = [{"Action": "run", "Package": PACKAGE, "Test": name}, {"Action": "pass", "Package": PACKAGE, "Test": name}]
        with self.assertRaises(gate.GateError): self.check()
    def test_incomplete_package_rejected(self):
        self.events.pop()
        with self.assertRaises(gate.GateError): self.check()

if __name__ == "__main__": unittest.main()
