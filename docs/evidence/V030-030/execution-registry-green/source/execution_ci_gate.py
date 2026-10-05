"""Strict fixed-identity gate for execution and return compatibility reports."""
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
        raise GateError("DOCTYPE is forbidden")

# Frozen Root case identities; supplied manifests cannot redefine acceptance.
_EXPECTED = {
  "suites": [
    {
      "classname": "org.weaveos.workflow.RootExecutionRegistryTest",
      "tests": [
        "allScopeBindingsAreCheckedBeforeAnyAction",
        "approvalModesTrackPartialCompletionAndInvalidateCancelledPeers(String)[1]",
        "approvalModesTrackPartialCompletionAndInvalidateCancelledPeers(String)[2]",
        "cancellationAfterExecutionCanOnlyReplayTheSuccessfulReceipt",
        "cancellationBeforeExecutionPermanentlyBlocksThatExactCommand",
        "concurrentDifferentCommandsOnTheSameSequenceApplyOnlyOne",
        "concurrentDuplicatesCommitOneEngineInstance",
        "concurrentExecutionAndCancellationHaveOneDurableOutcome",
        "conditionsUseTheLatestCommandRoutesRatherThanTheStartValues",
        "duplicatesReplayOriginalReceiptAfterProgressAndEngineRestart",
        "eachSuccessRecordsLatestVersionsWithoutChangingThePinnedDefinition",
        "everyPrecommitApprovalFailureRestoresOriginalTasksAndLedger(Stage)[1]",
        "everyPrecommitApprovalFailureRestoresOriginalTasksAndLedger(Stage)[2]",
        "everyPrecommitApprovalFailureRestoresOriginalTasksAndLedger(Stage)[3]",
        "everyPrecommitStartFailureRollsBackEngineLedgerAndTasks(Stage)[1]",
        "everyPrecommitStartFailureRollsBackEngineLedgerAndTasks(Stage)[2]",
        "everyPrecommitStartFailureRollsBackEngineLedgerAndTasks(Stage)[3]",
        "historicalApproverMayReturnOnlyToTheirOwnVisitedApprovalNode",
        "malformedPayloadOrHashMismatchCannotWriteAnything",
        "missingLookupNeverCreatesAReceiptOrCancellationProof",
        "noApprovalGraphCanCommitAnImmediatelyCompletedInstance",
        "outerRequiredRollbackNeverLeavesCommittedEngineOrReceipt",
        "registryRejectsWiringThatCouldCommitEngineAndLedgerSeparately",
        "rejectionEndsOnlyItsOwnInstance",
        "requestOwnsInputArraysAndReturnedTaskCollectionIsImmutable",
        "returnReactivatesVisitedNodeWithNewIdentitiesAndInvalidatesOldTasks",
        "sameCommandIdWithDifferentValidPayloadIsAConflict",
        "startCreatesDurableExactBindingsAndFullTaskMapping",
        "startIdentityAndDeploymentBindingsAreNotImplicitlyReused",
        "taskActorEpochAndCurrentVersionsAreChecked",
        "terminalInstanceRejectsNewCommandsButReplaysOldOnes",
        "unvisitedTargetIsRejectedWithoutMovingRuntime",
        "withdrawalRequiresOriginalInitiatorAndEnabledConfiguration"
      ]
    },
    {
      "classname": "org.weaveos.workflow.RootReturnCompatibilityTest",
      "tests": [
        "returnFromSecondMiRecreatesVisitedRosterAndResetsCounters(String)[1]",
        "returnFromSecondMiRecreatesVisitedRosterAndResetsCounters(String)[2]",
        "returnToCurrentPartiallyCompletedNodeStartsANewActivation",
        "rollbackAfterMovementRestoresExactPriorMiTasks",
        "withdrawalRollsBackThenRemovesAllRuntimeTasksButRetainsHistory"
      ]
    }
  ]
}

def validate(report_dir, manifest):
    if not isinstance(manifest, dict) or manifest != _EXPECTED:
        raise GateError("manifest must match the fixed 33+5 case contract")
    suites = manifest["suites"]
    try:
        reports = list(Path(report_dir).glob("TEST-*.xml"))
        expected_files = {f"TEST-{suite['classname']}.xml" for suite in suites}
        if len(reports) != 2 or {p.name for p in reports} != expected_files:
            raise GateError("exactly two expected reports are required")
        total = 0
        for suite in suites:
            root = ET.parse(Path(report_dir) / f"TEST-{suite['classname']}.xml",
                            parser=ET.XMLParser(target=_NoDoctypeBuilder())).getroot()
            if root.tag != "testsuite" or root.get("name") != suite["classname"]:
                raise GateError("unexpected suite")
            counts = {}
            for key in ("tests", "failures", "errors", "skipped"):
                value = root.get(key, "")
                if re.fullmatch(r"[0-9]+", value) is None:
                    raise GateError("invalid summary counter")
                counts[key] = int(value)
            cases = list(root.iter("testcase"))
            if cases != list(root.findall("testcase")):
                raise GateError("nested testcase")
            expected = Counter((suite["classname"], name) for name in suite["tests"])
            actual = Counter((case.get("classname"), case.get("name")) for case in cases)
            if actual != expected or counts["tests"] != len(cases):
                raise GateError("unexpected case identities or counts")
            for tag, key in (("failure", "failures"), ("error", "errors"), ("skipped", "skipped")):
                if counts[key] != 0 or next(root.iter(tag), None) is not None:
                    raise GateError("failure, error or skip")
            total += len(cases)
        return total
    except (OSError, ET.ParseError, ValueError) as error:
        raise GateError("report cannot be read or parsed") from error

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("report_dir", type=Path)
    parser.add_argument("manifest", type=Path)
    args = parser.parse_args()
    try:
        count = validate(args.report_dir, json.loads(args.manifest.read_text(encoding="utf-8")))
    except (GateError, OSError, UnicodeError, json.JSONDecodeError) as error:
        print(f"execution gate rejected: {error}", file=sys.stderr)
        return 1
    print(f"execution gate passed: {count} exact cases, no failures/errors/skips")
    return 0

if __name__ == "__main__":
    sys.exit(main())
