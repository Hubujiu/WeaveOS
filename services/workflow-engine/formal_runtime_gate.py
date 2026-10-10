"""Require complete, exact formal-process acceptance evidence without skips."""
import argparse
import json
from pathlib import Path
import sys

from rpc_ci_gate import GateError, _go

PACKAGE = "github.com/Hubujiu/WeaveOS/services/bff/cmd/bff"
CASES = {"TestRootFormalRuntimePublishAndApprove", "TestRootFormalRuntimeLostReplyBffRestart", "TestRootFormalRuntimeEngineRestart", "TestRootFormalRuntimeNodeSaveThenApproveLatest", "TestRootFormalRuntimeReturnThenWithdrawRecovery", "TestRootFormalRuntimeCloseWaitsForConfirmedProjection", "TestRootFormalRuntimeChildFailureDiagnosticsSurviveCleanup", "TestRootFormalRuntimeDeletionDrainsAndRecoversOriginalReceipt"}
EXPECTED = CASES | {"TestRootFormalRuntimePublishAndApprove/agree", "TestRootFormalRuntimePublishAndApprove/reject"}

def validate(path):
    _go(path, PACKAGE, CASES)
    try:
        with Path(path).open(encoding="utf-8") as report:
            observed = {event["Test"] for line in report if (event := json.loads(line)).get("Test")}
    except (OSError, UnicodeError, json.JSONDecodeError) as error:
        raise GateError("formal report cannot be read or parsed") from error
    if observed != EXPECTED:
        raise GateError("formal case identities must match exactly")
    return len(EXPECTED)

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("report", type=Path)
    args = parser.parse_args()
    try:
        count = validate(args.report)
    except GateError as error:
        print(f"formal runtime report rejected: {error}", file=sys.stderr)
        return 1
    print(f"formal runtime report passed: {count} exact cases, no failures or skips")
    return 0

if __name__ == "__main__":
    sys.exit(main())
