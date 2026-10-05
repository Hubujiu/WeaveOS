"""Require exact Root-owned RPC JUnit cases and genuine interop test2json completion."""
import argparse
from collections import Counter
import json
from pathlib import Path
import re
import sys
import xml.etree.ElementTree as ET

class GateError(Exception):
    pass

class _NoDoctype(ET.TreeBuilder):
    def doctype(self, name, public_id, system_id):
        raise GateError("DOCTYPE is forbidden")

def _manifest(manifest):
    if not isinstance(manifest, dict):
        raise GateError("invalid RPC manifest")
    classname, names = manifest.get("classname"), manifest.get("tests")
    package, go_names = manifest.get("go_package"), manifest.get("go_tests")
    if (not isinstance(classname, str) or re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*", classname) is None
            or not isinstance(names, list) or len(names) != 9
            or any(not isinstance(n, str) or not n for n in names) or len(set(names)) != 9
            or not isinstance(package, str) or not package or package.strip() != package
            or not isinstance(go_names, list) or len(go_names) != 1
            or any(not isinstance(n, str) or re.fullmatch(r"Test[A-Za-z0-9_]+", n) is None for n in go_names)):
        raise GateError("manifest must identify nine Java and one Go case")
    return classname, names, package, go_names

def _java(report_dir, classname, names):
    try:
        reports = list(Path(report_dir).glob("TEST-*.xml"))
        if len(reports) != 1 or reports[0].name != f"TEST-{classname}.xml":
            raise GateError("exactly one expected RPC report is required")
        root = ET.parse(reports[0], parser=ET.XMLParser(target=_NoDoctype())).getroot()
    except (OSError, ValueError, ET.ParseError) as error:
        raise GateError("RPC report cannot be read or parsed") from error
    if root.tag != "testsuite" or root.get("name") != classname:
        raise GateError("unexpected RPC suite")
    counts = {}
    for key in ("tests", "failures", "errors", "skipped"):
        value = root.get(key, "")
        if re.fullmatch(r"[0-9]+", value) is None:
            raise GateError("invalid RPC counter")
        try:
            counts[key] = int(value)
        except ValueError as error:
            raise GateError("invalid RPC counter") from error
    cases = list(root.iter("testcase"))
    if cases != list(root.findall("testcase")):
        raise GateError("RPC cases must be direct suite children")
    if Counter((c.get("classname"), c.get("name")) for c in cases) != Counter((classname, n) for n in names) or counts["tests"] != len(cases):
        raise GateError("RPC case identities must match exactly")
    for tag, key in (("failure", "failures"), ("error", "errors"), ("skipped", "skipped")):
        observed = sum(1 for _ in root.iter(tag))
        if counts[key] != observed or observed:
            raise GateError("RPC failures, errors or skips are forbidden")

def _go(go_json_path, package, expected):
    started = False
    completed = False
    cases = {}
    try:
        with Path(go_json_path).open(encoding="utf-8") as report:
            for line in report:
                event = json.loads(line)
                if not isinstance(event, dict) or event.get("Package") != package or completed:
                    raise GateError("unexpected interop event or package")
                action = event.get("Action")
                name = event.get("Test", "")
                if not isinstance(name, str) or action not in {"start", "run", "pass", "output", "pause", "cont"}:
                    raise GateError("invalid, failed or skipped interop event")
                if action == "start":
                    if started or name:
                        raise GateError("duplicate or invalid package start")
                    started = True
                    continue
                if not started:
                    raise GateError("interop package start is missing")
                if name:
                    parent = name.split("/", 1)[0]
                    if parent not in expected or (name != parent and parent not in cases):
                        raise GateError("unexpected interop case")
                    if action == "run":
                        if name in cases:
                            raise GateError("duplicate interop run")
                        cases[name] = "running"
                    elif action == "pass":
                        if cases.get(name) not in {"running", "paused"}:
                            raise GateError("missing run or duplicate interop completion")
                        cases[name] = "passed"
                    elif action == "pause":
                        if cases.get(name) != "running":
                            raise GateError("invalid interop pause")
                        cases[name] = "paused"
                    elif action == "cont":
                        if cases.get(name) != "paused":
                            raise GateError("invalid interop continuation")
                        cases[name] = "running"
                    elif name not in cases:
                        raise GateError("interop output precedes its run")
                elif action == "pass":
                    if any(cases.get(n) != "passed" for n in expected) or any(state != "passed" for state in cases.values()):
                        raise GateError("interop case completion is missing")
                    completed = True
                elif action != "output":
                    raise GateError("invalid package lifecycle")
                if action == "output" and not isinstance(event.get("Output"), str):
                    raise GateError("invalid interop output")
    except (OSError, UnicodeError, json.JSONDecodeError) as error:
        raise GateError("interop JSON cannot be read or parsed") from error
    if not started or not completed:
        raise GateError("interop package completion is missing")

def validate(report_dir, manifest, go_json_path):
    classname, names, package, go_names = _manifest(manifest)
    _java(report_dir, classname, names)
    _go(go_json_path, package, go_names)
    return len(names) + len(go_names)

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("report_dir", type=Path)
    parser.add_argument("manifest", type=Path)
    parser.add_argument("go_json_path", type=Path)
    args = parser.parse_args()
    try:
        count = validate(args.report_dir, json.loads(args.manifest.read_text(encoding="utf-8")), args.go_json_path)
    except (GateError, OSError, UnicodeError, json.JSONDecodeError) as error:
        print(f"workflow-rpc report gate rejected: {error}", file=sys.stderr)
        return 1
    print(f"workflow-rpc report gate passed: {count} exact cases, no failures/errors/skips")
    return 0

if __name__ == "__main__":
    sys.exit(main())
