"""Independent CI acceptance fixtures; these are not Java/PG proof results."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import patch
import xml.etree.ElementTree as ET

from ci_report import GateError, validate_reports
from ci_runner import run_gate

PACKAGE = Path(__file__).resolve().parent
MANIFEST = json.loads((PACKAGE / "expected-tests.json").read_text())
REPORT_NAME = "TEST-org.weaveos.proof.LocalCommandExecutorTest.xml"


def fixture_xml():
    suite = ET.Element("testsuite", name=MANIFEST["classname"], tests="15",
                       failures="0", errors="0", skipped="0")
    for name in MANIFEST["tests"]:
        ET.SubElement(suite, "testcase", classname=MANIFEST["classname"], name=name)
    return suite


class ReportGateTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.reports = Path(self.temp.name)
        self.started = time.time_ns()
        self.root = fixture_xml()

    def save(self):
        path = self.reports / REPORT_NAME
        ET.ElementTree(self.root).write(path, encoding="utf-8", xml_declaration=True)
        return path

    def rejects(self, exit_code=0):
        with self.assertRaises(GateError):
            validate_reports(self.reports, exit_code, self.started)

    def test_complete_fresh_suite_passes(self):
        self.save()
        result = validate_reports(self.reports, 0, self.started)
        self.assertEqual(15, result["tests"])
        self.assertEqual((0, 0, 0), (result["failures"], result["errors"], result["skipped"]))

    def test_missing_report_is_rejected(self):
        self.rejects()

    def test_skipped_case_is_rejected(self):
        self.root.set("skipped", "1")
        ET.SubElement(self.root[0], "skipped")
        self.save()
        self.rejects()

    def test_failure_is_rejected(self):
        self.root.set("failures", "1")
        ET.SubElement(self.root[0], "failure", message="intentional CI fixture assertion")
        self.save()
        self.rejects(1)

    def test_failure_even_with_zero_exit_is_rejected(self):
        ET.SubElement(self.root[0], "failure")
        self.save()
        self.rejects()

    def test_error_is_rejected(self):
        self.root.set("errors", "1")
        ET.SubElement(self.root[0], "error")
        self.save()
        self.rejects(1)

    def test_subset_is_rejected(self):
        self.root.remove(self.root[-1])
        self.root.set("tests", "14")
        self.save()
        self.rejects()

    def test_duplicate_cannot_replace_required_case(self):
        self.root[-1].set("name", self.root[0].get("name"))
        self.save()
        self.rejects()

    def test_other_class_is_rejected(self):
        self.root[0].set("classname", "other.Test")
        self.save()
        self.rejects()

    def test_unknown_case_is_rejected(self):
        self.root[0].set("name", "differentCase")
        self.save()
        self.rejects()

    def test_count_mismatch_is_rejected(self):
        self.root.set("tests", "16")
        self.save()
        self.rejects()

    def test_missing_counter_is_rejected(self):
        del self.root.attrib["skipped"]
        self.save()
        self.rejects()

    def test_malformed_counter_is_rejected(self):
        self.root.set("tests", "15.0")
        self.save()
        self.rejects()

    def test_malformed_xml_is_rejected(self):
        (self.reports / REPORT_NAME).write_text("<testsuite")
        self.rejects()

    def test_stale_report_is_rejected(self):
        path = self.save()
        os.utime(path, ns=(self.started - 10_000_000_000,) * 2)
        self.rejects()

    def test_extra_suite_is_rejected(self):
        self.save()
        ET.ElementTree(self.root).write(self.reports / "TEST-other.xml")
        self.rejects()

    def test_nonzero_exit_cannot_be_hidden_by_pass_report(self):
        self.save()
        self.rejects(5)


class RunnerExitTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.proof = Path(self.temp.name) / "proof"
        self.proof.mkdir()
        self.output = Path(self.temp.name) / "public"

    def command(self, kind="pass", exit_code=0):
        root = fixture_xml()
        if kind == "skip":
            root.set("skipped", "1")
            ET.SubElement(root[0], "skipped")
        if kind == "failure":
            root.set("failures", "1")
            ET.SubElement(root[0], "failure", message="intentional CI fixture assertion")
        xml = ET.tostring(root, encoding="unicode")
        source = "from pathlib import Path\nimport sys\n"
        if kind != "missing":
            source += "p=Path('target/surefire-reports'); p.mkdir(parents=True,exist_ok=True)\n"
            source += f"(p/{REPORT_NAME!r}).write_text({xml!r})\n"
        source += f"print('controlled runner fixture: {kind}')\nsys.exit({exit_code})\n"
        return [sys.executable, "-c", source]

    def result(self):
        path = self.output / "result.json"
        self.assertTrue(path.is_file(), "runner must preserve a public verdict")
        return json.loads(path.read_text())

    def test_pass_records_real_exit_and_fresh_reports(self):
        self.assertEqual(0, run_gate(self.proof, self.output, self.command()))
        result = self.result()
        self.assertEqual("passed", result["status"])
        self.assertEqual(0, result["maven_exit"])
        self.assertEqual(15, result["summary"]["tests"])
        self.assertTrue((self.output / "reports" / REPORT_NAME).is_file())

    def test_fresh_child_report_survives_a_wall_clock_ahead_of_filesystem(self):
        # Both output deletion and the child run are real; only the wall clock is advanced.
        with patch("ci_runner.time.time_ns", return_value=time.time_ns() + 1_000_000_000):
            self.assertEqual(0, run_gate(self.proof, self.output, self.command()))

    def test_assertion_failure_is_nonzero_and_preserves_report(self):
        self.assertNotEqual(0, run_gate(self.proof, self.output, self.command("failure", 1)))
        self.assertEqual(1, self.result()["maven_exit"])
        self.assertTrue((self.output / "reports" / REPORT_NAME).is_file())

    def test_missing_report_is_nonzero_even_if_child_exits_zero(self):
        self.assertNotEqual(0, run_gate(self.proof, self.output, self.command("missing")))

    def test_skip_is_nonzero_even_if_child_exits_zero(self):
        self.assertNotEqual(0, run_gate(self.proof, self.output, self.command("skip")))

    def test_nonzero_child_exit_is_preserved(self):
        self.assertNotEqual(0, run_gate(self.proof, self.output, self.command(exit_code=5)))
        self.assertEqual(5, self.result()["maven_exit"])

    def test_previous_report_cannot_survive_a_missing_new_report(self):
        reports = self.proof / "target" / "surefire-reports"
        reports.mkdir(parents=True)
        ET.ElementTree(fixture_xml()).write(reports / REPORT_NAME)
        self.assertNotEqual(0, run_gate(self.proof, self.output, self.command("missing")))
        self.assertFalse((self.output / "reports" / REPORT_NAME).exists())

    def test_fixture_assertion_process_really_fails(self):
        result = subprocess.run([sys.executable, "-c", "assert False, 'intentional CI oracle'"],
                                capture_output=True, text=True)
        self.assertNotEqual(0, result.returncode)
        self.assertIn("AssertionError: intentional CI oracle", result.stderr)


class WorkflowGateTests(unittest.TestCase):
    def test_independent_workflow_runs_tests_and_guarded_proof(self):
        workflow = PACKAGE.parents[1] / ".github/workflows/java-flowable-proof.yml"
        self.assertTrue(workflow.is_file(), "Java-specific CI wiring is missing")
        text = workflow.read_text()
        self.assertIn("python3 prototypes/flowable-local-tx/ci_tests.py", text)
        self.assertIn("python3 prototypes/flowable-local-tx/ci_runner.py", text)
        self.assertIn("if: always()", text)
        self.assertNotIn("continue-on-error", text)
        self.assertNotIn("workflow_call:", text)
        self.assertNotIn("pull_request_target:", text)
        self.assertNotIn("secrets.", text)


if __name__ == "__main__":
    unittest.main(verbosity=2)
