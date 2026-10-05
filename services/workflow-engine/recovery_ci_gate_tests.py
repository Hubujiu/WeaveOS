"""Root-owned acceptance for real worker report identity and completion."""
import copy
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("recovery_gate", Path(__file__).with_name("recovery_ci_gate.py"))
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)
PACKAGE = "github.com/Hubujiu/WeaveOS/services/bff/internal/apprecordservice"
CASES = ["TestRootRecoveryJavaLifecycleInterop", "TestRootRecoveryJavaDroppedResponseInterop", "TestRootRecoveryJavaApplicationCommitFaultInterop"]

class RootRecoveryGateTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.path = Path(self.temp.name) / "interop.jsonl"
        self.events = [{"Action": "start", "Package": PACKAGE}]
        for name in CASES:
            self.events.extend([{"Action": "run", "Package": PACKAGE, "Test": name}, {"Action": "pass", "Package": PACKAGE, "Test": name}])
        self.events.append({"Action": "pass", "Package": PACKAGE})
    def write(self):
        self.path.write_text("".join(json.dumps(x) + "\n" for x in self.events))
    def check(self):
        self.write()
        return gate.validate(self.path)
    def test_all_exact_cases_pass(self):
        self.assertEqual(3, self.check())
    def test_missing_file_fails(self):
        with self.assertRaises(gate.GateError):
            gate.validate(self.path)
    def test_empty_or_malformed_report_fails(self):
        for text in ("", "not json\n", "[]\n"):
            self.path.write_text(text)
            with self.assertRaises(gate.GateError):
                gate.validate(self.path)
    def test_each_missing_case_fails(self):
        original = copy.deepcopy(self.events)
        for name in CASES:
            self.events = [x for x in original if x.get("Test") != name]
            with self.assertRaises(gate.GateError):
                self.check()
    def test_each_case_cannot_skip_or_fail(self):
        for index in (2, 4, 6):
            for action in ("skip", "fail"):
                self.events[index]["Action"] = action
                with self.assertRaises(gate.GateError):
                    self.check()
            self.events[index]["Action"] = "pass"
    def test_package_start_and_completion_required(self):
        original = copy.deepcopy(self.events)
        for events in (original[1:], original[:-1]):
            self.events = events
            with self.assertRaises(gate.GateError):
                self.check()
    def test_duplicate_run_or_pass_fails(self):
        original = copy.deepcopy(self.events)
        for index in (1, 2):
            self.events = copy.deepcopy(original)
            self.events.insert(index + 1, copy.deepcopy(self.events[index]))
            with self.assertRaises(gate.GateError):
                self.check()
    def test_unknown_case_and_wrong_package_fail(self):
        original = copy.deepcopy(self.events)
        for field, value in (("Test", "TestSomeOtherCase"), ("Package", "wrong/package")):
            self.events = copy.deepcopy(original)
            self.events[2][field] = value
            with self.assertRaises(gate.GateError):
                self.check()
    def test_data_after_completion_fails(self):
        self.events.append({"Action": "output", "Package": PACKAGE, "Output": "extra"})
        with self.assertRaises(gate.GateError):
            self.check()
    def test_output_cannot_replace_actual_test_run(self):
        self.events[1] = {"Action": "output", "Package": PACKAGE, "Output": "PASS " + CASES[0]}
        with self.assertRaises(gate.GateError):
            self.check()
    def test_run_without_terminal_fails(self):
        del self.events[2]
        with self.assertRaises(gate.GateError):
            self.check()

if __name__ == "__main__":
    unittest.main()
