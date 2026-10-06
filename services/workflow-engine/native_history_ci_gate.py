"""Require the exact eight Root-owned native-history cases with no skips."""
import argparse
import sys
from rpc_ci_gate import GateError, _java
CLASS = 'org.weaveos.workflow.RootNativeHistoryTest'
EXPECTED = ('startWritesNativeHistorySynchronouslyWithoutShadowVisits', 'visitedReturnWorksWithoutLegacyRows', 'forgedLegacyVisitCannotAuthorizeUnvisitedTarget', 'reopenedEngineUsesNativeHistoryAndPreservesOldReceipt', 'repeatedReturnsKeepDistinctActivationsAndAppendHistory', 'failedReturnRollsBackNativeHistoryAndExactRuntimeTasks', 'missingOrAsyncHistoryConfigurationCannotEnableBridge', 'anotherInstanceHistoryCannotAuthorizeCurrentReturn')

def validate(report_dir):
    _java(report_dir, CLASS, EXPECTED)
    return len(EXPECTED)

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('report_dir')
    args = parser.parse_args()
    try:
        count = validate(args.report_dir)
    except GateError as error:
        print(f'native-history gate rejected: {error}', file=sys.stderr)
        return 1
    print(f'native-history gate passed: {count} exact cases, no failures/errors/skips')
    return 0

if __name__ == '__main__':
    sys.exit(main())
