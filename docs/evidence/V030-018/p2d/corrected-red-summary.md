# Corrected P2d RED

Root's fixture correction was cherry-picked as `ac4edc4`; it only changes the text field's configuration to include `maxLength: null`. The seven top-level `TestRootWorkflowSchema*` cases now pass shared HTTP/PostgreSQL/Redis setup and reach business assertions.

Five assertions are RED because incompatible changes are currently accepted: `TestRootWorkflowSchemaSaveRejectsAndRollsBackAllChanges` gets HTTP 200 instead of 409 and commits the replacement; `TestRootWorkflowSchemaPreflightReportsConflictWithoutConfirmation` reports `saveAllowed: true` without `workflowConflicts`; `TestRootWorkflowSchemaRechecksAfterPreviouslyAllowedPreflight` gets HTTP 200 after the definition becomes enabled; `TestRootWorkflowSchemaInFlightOldDefinitionStillProtectsField` gets HTTP 200; `TestRootWorkflowSchemaSaveSerializesWithEnable` observes Save wait behind Enable but then receives HTTP 200 instead of the incompatibility conflict.

Compatible rename/text-to-multiline subtests and `TestRootWorkflowSchemaClosedUnusedHistoryAllowsRemoval` pass. This is a valid business RED; no Root test assertions were edited. The earlier seven shared-setup HTTP 400 failures remain preserved in `red-output.txt` as invalid preparation evidence.
