"""Require three real HTTPS/Java scenarios plus both action variants without skips."""
import argparse
import importlib.util
from pathlib import Path
import sys

_spec = importlib.util.spec_from_file_location("action_legacy_rpc_gate", Path(__file__).with_name("rpc_ci_gate.py"))
_legacy = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_legacy)
GateError = _legacy.GateError
_PACKAGE = "github.com/Hubujiu/WeaveOS/services/bff/cmd/bff"
_EXPECTED = ("TestRootWorkflowActionJavaHTTPSAgreeRejectInterop", "TestRootWorkflowActionJavaLostReplyKeepsPendingUntilRecoveryInterop", "TestRootWorkflowActionJavaApplicationFaultRecoversOriginalReceiptInterop", "TestRootWorkflowActionJavaHTTPSAgreeRejectInterop/agree", "TestRootWorkflowActionJavaHTTPSAgreeRejectInterop/reject")

def validate(path):
    _legacy._go(path, _PACKAGE, _EXPECTED)
    return len(_EXPECTED)

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("report", type=Path)
    args = parser.parse_args()
    try:
        count = validate(args.report)
    except GateError as error:
        print(f"workflow action HTTP report rejected: {error}", file=sys.stderr)
        return 1
    print(f"workflow action HTTP report passed: {count} exact cases, no failures or skips")
    return 0

if __name__ == "__main__":
    sys.exit(main())
