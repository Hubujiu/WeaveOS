"""Require the complete Root-owned workflow-engine Surefire case manifest."""
import argparse
from collections import Counter
import json
from pathlib import Path
import re
import sys
import xml.etree.ElementTree as ET


class GateError(Exception):
    pass


class _NoDoctypeBuilder(ET.TreeBuilder):
    def doctype(self, name, public_id, system_id):
        raise GateError("DOCTYPE is forbidden in a test report")


def validate(report_dir, manifest):
    if not isinstance(manifest, dict):
        raise GateError("invalid expected case manifest")
    classname = manifest.get("classname")
    names = manifest.get("tests")
    if (not isinstance(classname, str)
            or re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*", classname) is None
            or not isinstance(names, list) or len(names) != 18
            or any(not isinstance(name, str) or not name for name in names)
            or len(set(names)) != 18):
        raise GateError("manifest must identify 18 distinct expected cases")
    expected = Counter((classname, name) for name in names)
    try:
        reports = list(Path(report_dir).glob("TEST-*.xml"))
        if len(reports) != 1 or reports[0].name != f"TEST-{classname}.xml":
            raise GateError("exactly one expected Surefire report is required")
        parser = ET.XMLParser(target=_NoDoctypeBuilder())
        root = ET.parse(reports[0], parser=parser).getroot()
    except (OSError, ET.ParseError, ValueError) as error:
        raise GateError("test report cannot be read or parsed") from error
    if root.tag != "testsuite" or root.get("name") != classname:
        raise GateError("unexpected test suite")
    counts = {}
    for key in ("tests", "failures", "errors", "skipped"):
        value = root.get(key, "")
        if re.fullmatch(r"[0-9]+", value) is None:
            raise GateError("invalid test summary counter")
        try:
            counts[key] = int(value)
        except ValueError as error:
            raise GateError("invalid test summary counter") from error
    cases = list(root.iter("testcase"))
    if cases != list(root.findall("testcase")):
        raise GateError("test cases must be direct suite children")
    actual = Counter((case.get("classname"), case.get("name")) for case in cases)
    if actual != expected or counts["tests"] != len(cases):
        raise GateError("test identities must match all 18 expected cases exactly")
    for tag, key in (("failure", "failures"), ("error", "errors"), ("skipped", "skipped")):
        observed = sum(1 for _ in root.iter(tag))
        if counts[key] != observed or observed != 0:
            raise GateError("test report contains or miscounts failures, errors or skips")
    return len(cases)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("report_dir", type=Path)
    parser.add_argument("manifest", type=Path)
    args = parser.parse_args()
    try:
        manifest = json.loads(args.manifest.read_text(encoding="utf-8"))
        count = validate(args.report_dir, manifest)
    except (GateError, OSError, UnicodeError, json.JSONDecodeError) as error:
        print(f"workflow-engine report gate rejected: {error}", file=sys.stderr)
        return 1
    print(f"workflow-engine report gate passed: {count} exact cases, no failures/errors/skips")
    return 0


if __name__ == "__main__":
    sys.exit(main())
