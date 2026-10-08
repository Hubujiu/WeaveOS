"""Root-owned synthetic report tests. No network or real databases."""
import copy
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
import xml.etree.ElementTree as ET

BASE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("gate", BASE / "execution_ci_gate.py")
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)

class RootExecutionGateTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.path = Path(self.tmp.name)
        self.manifest = json.loads((BASE / "expected-execution-tests.json").read_text())
        self.roots = []
        for suite in self.manifest["suites"]:
            root = ET.Element("testsuite", name=suite["classname"], tests=str(len(suite["tests"])),
                              failures="0", errors="0", skipped="0")
            for name in suite["tests"]:
                ET.SubElement(root, "testcase", name=name, classname=suite["classname"])
            self.roots.append(root)

    def write(self):
        for i, root in enumerate(self.roots):
            filename = "TEST-" + self.manifest["suites"][i]["classname"] + ".xml"
            ET.ElementTree(root).write(self.path / filename)

    def check(self):
        self.write()
        return gate.validate(self.path, self.manifest)

    def test_exact_36_and_5_pass(self):
        self.assertEqual(41, self.check())

    def test_no_reports_fails(self):
        with self.assertRaises(gate.GateError):
            gate.validate(self.path, self.manifest)

    def test_missing_either_suite_fails(self):
        for i in range(2):
            self.write()
            (self.path / ("TEST-" + self.manifest["suites"][i]["classname"] + ".xml")).unlink()
            with self.assertRaises(gate.GateError):
                gate.validate(self.path, self.manifest)

    def test_extra_report_fails(self):
        self.write()
        (self.path / "TEST-unexpected.xml").write_text("<testsuite/>")
        with self.assertRaises(gate.GateError):
            gate.validate(self.path, self.manifest)

    def test_wrong_report_filename_fails(self):
        self.write()
        first = next(self.path.glob("TEST-*.xml"))
        first.rename(self.path / "TEST-renamed.xml")
        with self.assertRaises(gate.GateError):
            gate.validate(self.path, self.manifest)

    def test_missing_case_fails(self):
        self.roots[0].remove(self.roots[0][0])
        self.roots[0].set("tests", "35")
        with self.assertRaises(gate.GateError):
            self.check()

    def test_duplicate_replacement_case_fails(self):
        self.roots[0][1].set("name", self.roots[0][0].get("name"))
        with self.assertRaises(gate.GateError):
            self.check()

    def test_case_class_mismatch_fails(self):
        self.roots[1][0].set("classname", self.manifest["suites"][0]["classname"])
        with self.assertRaises(gate.GateError):
            self.check()

    def test_extra_case_fails(self):
        ET.SubElement(self.roots[0], "testcase", name="unexpected", classname=self.roots[0].get("name"))
        self.roots[0].set("tests", str(len(self.roots[0])))
        self.assertEqual(37, len(self.roots[0]), "extra identity is the only invalid condition")
        with self.assertRaises(gate.GateError):
            self.check()

    def test_failure_error_skip_cannot_hide_behind_zero_summary(self):
        for root in self.roots:
            for tag in ("failure", "error", "skipped"):
                child = ET.SubElement(root[0], tag)
                with self.assertRaises(gate.GateError):
                    self.check()
                root[0].remove(child)

    def test_summary_counts_are_strict(self):
        for key, value in (("tests", "32"), ("tests", "-1"), ("tests", "38"),
                           ("tests", "bad"), ("errors", "1"), ("failures", "1"), ("skipped", "1")):
            before = self.roots[0].get(key)
            self.roots[0].set(key, value)
            with self.assertRaises(gate.GateError):
                self.check()
            self.roots[0].set(key, before)

    def test_nested_case_fails(self):
        case = self.roots[0][0]
        self.roots[0].remove(case)
        ET.SubElement(self.roots[0], "wrapper").append(case)
        with self.assertRaises(gate.GateError):
            self.check()

    def test_wrong_suite_root_and_name_fail(self):
        before = self.roots[0].get("name")
        self.roots[0].set("name", "other")
        with self.assertRaises(gate.GateError):
            self.check()
        self.roots[0].set("name", before)
        self.roots[0].tag = "testsuites"
        with self.assertRaises(gate.GateError):
            self.check()

    def test_malformed_report_fails(self):
        self.write()
        next(self.path.glob("TEST-*.xml")).write_text("<broken")
        with self.assertRaises(gate.GateError):
            gate.validate(self.path, self.manifest)

    def test_doctype_is_rejected(self):
        self.write()
        first = next(self.path.glob("TEST-*.xml"))
        first.write_text('<!DOCTYPE testsuite [<!ENTITY x "bad">]>' + first.read_text())
        with self.assertRaises(gate.GateError):
            gate.validate(self.path, self.manifest)

    def test_manifest_must_have_exact_two_distinct_classes_and_36_5_cases(self):
        self.write()
        bads = [None, {}, {"suites": []}]
        for mutation in ("duplicate_class", "duplicate_case", "drop_case", "rename_class"):
            bad = copy.deepcopy(self.manifest)
            if mutation == "duplicate_class":
                bad["suites"][1]["classname"] = bad["suites"][0]["classname"]
            elif mutation == "duplicate_case":
                bad["suites"][0]["tests"][1] = bad["suites"][0]["tests"][0]
            elif mutation == "drop_case":
                bad["suites"][0]["tests"].pop()
            else:
                bad["suites"][0]["classname"] = "Unexpected"
            bads.append(bad)
        for bad in bads:
            with self.assertRaises(gate.GateError):
                gate.validate(self.path, bad)

if __name__ == "__main__":
    unittest.main()
