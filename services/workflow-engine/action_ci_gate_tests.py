"""Root-owned exact HTTPS/Java action evidence gate tests."""
import copy
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
spec = importlib.util.spec_from_file_location("action_gate", Path(__file__).with_name("action_ci_gate.py"))
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)
PACKAGE = "github.com/Hubujiu/WeaveOS/services/bff/cmd/bff"
CASES = ["TestRootWorkflowActionJavaHTTPSAgreeRejectInterop","TestRootWorkflowActionJavaLostReplyKeepsPendingUntilRecoveryInterop","TestRootWorkflowActionJavaApplicationFaultRecoversOriginalReceiptInterop"]
EXPECTED = CASES + [CASES[0] + "/agree", CASES[0] + "/reject"]
class RootActionGateTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.path = Path(self.temp.name) / "actions.jsonl"
        self.events = [{"Action": "start", "Package": PACKAGE}]
        for name in CASES:
            self.events.append({"Action": "run", "Package": PACKAGE, "Test": name})
            if name == CASES[0]:
                for action in ("agree", "reject"):
                    child = name + "/" + action
                    self.events.extend([{"Action": "run", "Package": PACKAGE, "Test": child}, {"Action": "pass", "Package": PACKAGE, "Test": child}])
            self.events.append({"Action": "pass", "Package": PACKAGE, "Test": name})
        self.events.append({"Action": "pass", "Package": PACKAGE})
    def check(self):
        self.path.write_text("".join(json.dumps(x) + "\n" for x in self.events))
        return gate.validate(self.path)
    def test_exact_three_scenarios_and_two_variants(self):
        self.assertEqual(5, self.check())
    def test_each_missing_scenario_or_action_rejected(self):
        original = copy.deepcopy(self.events)
        for name in EXPECTED:
            self.events = [x for x in original if x.get("Test") != name]
            with self.assertRaises(gate.GateError): self.check()
    def test_any_failed_or_skipped_case_rejected(self):
        original = copy.deepcopy(self.events)
        for i, event in enumerate(original):
            if event.get("Test") and event["Action"] == "pass":
                for action in ("skip", "fail"):
                    self.events = copy.deepcopy(original)
                    self.events[i]["Action"] = action
                    with self.assertRaises(gate.GateError): self.check()
    def test_missing_start_or_completion_rejected(self):
        original = copy.deepcopy(self.events)
        for events in (original[1:], original[:-1]):
            self.events = events
            with self.assertRaises(gate.GateError): self.check()
    def test_duplicate_lifecycle_rejected(self):
        original = copy.deepcopy(self.events)
        for i in (1, 2, 3):
            self.events = copy.deepcopy(original)
            self.events.insert(i + 1, copy.deepcopy(self.events[i]))
            with self.assertRaises(gate.GateError): self.check()
    def test_wrong_package_and_unexpected_top_case_rejected(self):
        original = copy.deepcopy(self.events)
        for field, value in (("Package", "wrong/package"), ("Test", "TestUnrelated")):
            self.events = copy.deepcopy(original)
            self.events[1][field] = value
            with self.assertRaises(gate.GateError): self.check()
    def test_printed_pass_is_not_actual_completion(self):
        self.events[3] = {"Action": "output", "Package": PACKAGE, "Test": EXPECTED[3], "Output": "PASS"}
        with self.assertRaises(gate.GateError): self.check()
    def test_trailing_event_rejected(self):
        self.events.append({"Action": "output", "Package": PACKAGE, "Output": "extra"})
        with self.assertRaises(gate.GateError): self.check()
    def test_missing_or_malformed_file_rejected(self):
        with self.assertRaises(gate.GateError): gate.validate(self.path)
        for text in ("", "bad json\n", "[]\n"):
            self.path.write_text(text)
            with self.assertRaises(gate.GateError): gate.validate(self.path)
if __name__ == "__main__": unittest.main()
