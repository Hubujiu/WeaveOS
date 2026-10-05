"""Require frozen Root execution RPC cases and genuine successful report lifecycles."""
import argparse
import importlib.util
import json
from pathlib import Path
import sys

_spec = importlib.util.spec_from_file_location("execution_legacy_rpc_gate", Path(__file__).with_name("rpc_ci_gate.py"))
_legacy = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_legacy)
GateError = _legacy.GateError

# Frozen independently from the supplied manifest.
_EXPECTED = {'classname': 'org.weaveos.workflow.RootExecutionGrpcIT', 'tests': ['executeReturnsCommittedBoundReceipt', 'duplicateExecutionReplaysExactReceipt', 'conflictingCommandCannotReuseIdentity', 'missingLookupIsExplicitAndDoesNotCancel', 'lookupRejectsDifferentFingerprint', 'cancellationFirstDurablyPreventsExecution', 'executionFirstCannotBecomeCancelled', 'invalidRequestsNeverWriteLedger', 'databaseFailureRollsBackAndHidesPrivateDetails', 'everyMethodRejectsProvisionalOuterTransaction', 'lostReplyThenServiceRestartRecoversCommittedReceipt'], 'go_package': 'github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc', 'go_unit_tests': ['TestRootExecutionClientConstructorBounds', 'TestRootExecutionClientSendsCanonicalBoundBytes', 'TestRootExecutionClientRejectsInputsBeforeRPC', 'TestRootExecutionClientNilReceiverReturnsError', 'TestRootExecutionClientRejectsDamagedReceipts', 'TestRootExecutionClientLookupStatesAndBinding', 'TestRootExecutionClientCancellationMayRecoverSuccess', 'TestRootExecutionClientTransportErrorsStayUnknown', 'TestRootExecutionClientDeadlineAndParentCancellation', 'TestRootExecutionClientDoesNotMutateCallerInput'], 'go_interop_tests': ['TestRootGoJavaPostgresExecutionLifecycleInterop', 'TestRootGoJavaPostgresExecutionCancellationInterop', 'TestRootGoJavaPostgresExecutionReturnAndWithdrawInterop']}

def validate(report_dir, manifest, unit_json, interop_json):
    if not isinstance(manifest, dict) or set(manifest) != set(_EXPECTED):
        raise GateError("invalid execution RPC manifest")
    for key, expected in _EXPECTED.items():
        actual = manifest[key]
        if isinstance(expected, list):
            if not isinstance(actual, list) or any(not isinstance(n, str) for n in actual) or len(actual) != len(expected) or set(actual) != set(expected):
                raise GateError("execution RPC case identities must remain frozen")
        elif actual != expected:
            raise GateError("execution RPC suite and package must remain frozen")
    _legacy._java(report_dir, _EXPECTED["classname"], _EXPECTED["tests"])
    _legacy._go(unit_json, _EXPECTED["go_package"], _EXPECTED["go_unit_tests"])
    _legacy._go(interop_json, _EXPECTED["go_package"], _EXPECTED["go_interop_tests"])
    return 24

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("report_dir", type=Path)
    parser.add_argument("manifest", type=Path)
    parser.add_argument("unit_json", type=Path)
    parser.add_argument("interop_json", type=Path)
    args = parser.parse_args()
    try:
        count = validate(args.report_dir, json.loads(args.manifest.read_text(encoding="utf-8")), args.unit_json, args.interop_json)
    except (GateError, OSError, UnicodeError, json.JSONDecodeError) as error:
        print(f"execution-rpc report gate rejected: {error}", file=sys.stderr)
        return 1
    print(f"execution-rpc report gate passed: {count} exact cases, no failures/errors/skips")
    return 0

if __name__ == "__main__":
    sys.exit(main())
