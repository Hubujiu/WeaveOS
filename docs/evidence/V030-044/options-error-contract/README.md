# Safe discovery error contract repair

Source: V044 PRD/ADR/P2c body-only plan "第七切片错误合同补全计划", fetched and read back 2026-10-09 11:24 UTC. No new runtime behavior.

- Base ef343c7da81fb4e7bf44f13dd746c6f5509ea168.
- Independent contract tests first failed on missing WORKFLOW_NOT_READY and misspelled expiry description (2 pass / 2 fail). Exact test patch and raw RED are retained; test commit 8766026 precedes schema repair.
- Added only dedicated WorkflowManualOptionsErrorEnvelope, leaving ordinary RecordErrorEnvelope unchanged. Discovery 409 points at it; description uses existing APPLICATION_QUERY_CONTEXT_EXPIRED.
- Node contracts/governance/foundation: 520 pass, zero failures/skips.
- Redocly 2.54.2 telemetry/update-check disabled: valid, 13 warnings. Rechecking the unchanged base spec with the same installed tool also returns 13 warnings, so earlier 12-warning reports do not describe this invocation. No warnings suppressed.
- No Go/Java/DB/runtime source changes. Exact remote CI still required. Remote ef343 browser failed; not a fully passing baseline.
