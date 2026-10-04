"""Require this run's complete, successful controlled Java proof report."""
from collections import Counter
import hashlib
import json
from pathlib import Path
import re
import xml.etree.ElementTree as ET


class GateError(ValueError):
    pass


def validate_reports(report_dir, maven_exit, started_ns):
    if maven_exit != 0:
        raise GateError(f"proof process exited {maven_exit}")
    manifest = json.loads(Path(__file__).with_name("expected-tests.json").read_text())
    expected = Counter((manifest["classname"], name) for name in manifest["tests"])
    if len(expected) != 15 or any(count != 1 for count in expected.values()):
        raise GateError("expected manifest must contain 15 distinct cases")
    reports = list(Path(report_dir).glob("TEST-*.xml"))
    expected_file = f"TEST-{manifest['classname']}.xml"
    if len(reports) != 1 or reports[0].name != expected_file:
        raise GateError("exactly one expected JUnit report is required")
    report = reports[0]
    if report.stat().st_mtime_ns < started_ns:
        raise GateError("JUnit report predates this proof run")
    try:
        root = ET.parse(report).getroot()
    except (ET.ParseError, OSError) as error:
        raise GateError("JUnit report cannot be parsed") from error
    if root.tag != "testsuite" or root.get("name") != manifest["classname"]:
        raise GateError("unexpected JUnit suite")
    counters = {}
    for key in ("tests", "failures", "errors", "skipped"):
        value = root.get(key, "")
        if re.fullmatch(r"[0-9]+", value) is None:
            raise GateError(f"invalid JUnit {key} counter")
        counters[key] = int(value)
    cases = list(root.iter("testcase"))
    actual = Counter((case.get("classname"), case.get("name")) for case in cases)
    if actual != expected or counters["tests"] != len(cases):
        raise GateError("JUnit cases must match the complete 15-case manifest")
    for tag, key in (("failure", "failures"), ("error", "errors"), ("skipped", "skipped")):
        observed = sum(1 for _ in root.iter(tag))
        if counters[key] != observed or observed != 0:
            raise GateError(f"JUnit contains or miscounts {key}")
    return {**counters, "report_sha256": hashlib.sha256(report.read_bytes()).hexdigest(),
            "cases": sorted(name for _, name in actual)}
