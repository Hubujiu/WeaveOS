"""Run the fixed proof and preserve diagnostics without masking its verdict."""
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import time

from ci_report import GateError, validate_reports


def run_gate(proof_dir, output_dir, command):
    reports = proof_dir / "target" / "surefire-reports"
    # Clear old reports before invoking Maven, including when Maven itself fails early.
    if reports.exists():
        shutil.rmtree(reports)
    if output_dir.exists():
        shutil.rmtree(output_dir)
    output_dir.mkdir(parents=True)
    wall_started_ns = time.time_ns()
    marker = output_dir / "run-start.marker"
    marker.touch()
    started_ns = marker.stat().st_mtime_ns
    result = {"status": "failed", "started_at": datetime.now(timezone.utc).isoformat(),
              "started_ns": started_ns, "wall_started_ns": wall_started_ns, "maven_exit": None,
              "head_sha": os.environ.get("B3_CI_HEAD_SHA"),
              "checkout_sha": os.environ.get("B3_CI_CHECKOUT_SHA")}
    try:
        with (output_dir / "proof-output.txt").open("w") as log:
            with subprocess.Popen(command, cwd=proof_dir, stdout=subprocess.PIPE,
                                  stderr=subprocess.STDOUT, text=True) as process:
                for line in process.stdout:
                    log.write(line)
                    print(line, end="", flush=True)
                result["maven_exit"] = process.wait()
        result["summary"] = validate_reports(reports, result["maven_exit"], started_ns)
        result["status"] = "passed"
    except (GateError, OSError) as error:
        result["reason"] = str(error)
        print(f"Java proof gate rejected: {error}", file=sys.stderr)
    finally:
        copied = output_dir / "reports"
        copied.mkdir()
        for report in reports.glob("TEST-*.xml"):
            shutil.copy2(report, copied / report.name)
        result["finished_at"] = datetime.now(timezone.utc).isoformat()
        (output_dir / "result.json").write_text(json.dumps(result, indent=2) + "\n")
    return 0 if result["status"] == "passed" else 1


if __name__ == "__main__":
    package = Path(__file__).resolve().parent
    sys.exit(run_gate(package, package / ".work/java-ci/proof",
                      [str(package / "run-proof.sh"), "clean", "verify"]))
