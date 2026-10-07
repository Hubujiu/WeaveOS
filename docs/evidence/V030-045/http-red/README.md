# Real lifecycle HTTPS ingress RED

Frozen source681dfee8f4cc5b9ed22f26d95b0e914246a3e4ee. Exact command: `go test -race -p 1 -count=1 -json -run '^TestRootLifecycleHTTPS' ./cmd/bff`. Exit1; four top-level and two withdraw/return subcases FAIL, no PASS/SKIP. Actual valid preview requests expect200 and return404 API_NOT_FOUND, before public lifecycle routes are wired. No compilation/environment failure; test stderr empty. Deeper Accept/operation receipt, closed body/query, identity/CSRF and stale-preview assertions were not reached.

| Root case | Actual failing assertion |
| --- | --- |
| PreviewAcceptAndOperation/withdraw and /return | L17: want200 actual404 API_NOT_FOUND |
| RejectsClosedBodyAndQuery | L44: want200 actual404 API_NOT_FOUND |
| IdentityAndCSRF | L59: want200 actual404 API_NOT_FOUND |
| OldPreviewCannotApproveChangedInstance | L73: want200 actual404 API_NOT_FOUND |

Before testing, real psql verifies original V045 database weaveos_b2_isolated_test and Goose version22, with Goose status separately captured. This reuses the existing authorized PG18.6/Redis8.2.10 fixture after V046 completes on its independent hot21/archive6 databases. No database migration/reset/Down performed, and both environments retained. Exact commands, times, cases and hashes are in original run.json. Source/test/dependency hashes unchanged, tree clean after execution.

Root frozen HTTPS test SHA256:1b50a59eaac41c98af1ed4c5966ff6df8c8bed17c92f1acb19b346d948ff596c. Original JSONL, stdout/stderr, exits, run.json and source snapshots copied byte-for-byte. No executor production/test/debug edits, merge or deployment; TDD:N/A for evidence-only preservation. Real Flowable is not covered by this run.

| Original artifact | Bytes | SHA256 |
| --- | --- | --- |
| database-and-goose-version.exit | 2 | 9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa |
| database-and-goose-version.stderr | 0 | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| database-and-goose-version.stdout | 28 | da1ad12b05fe2d4fbb09917ea04e1f77237a0d6422314cf2fd548a8adc2738df |
| go.mod | 759 | aed642571afb8f38fedf75b2ca402f1464cd577799b9ea8cb7c6fd05072a2632 |
| go.sum | 5039 | 66c155748037a2c08e671e844ab359e500b86775a3304a86bea239835b3da99f |
| goose-status.exit | 2 | 9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa |
| goose-status.stderr | 1937 | 75f7b9a5fe0c36e4175b82040c250ea4475f1c0aba5375349bdea2b1f7b0c5d0 |
| goose-status.stdout | 0 | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| lifecycle-https.exit | 2 | 4355a46b19d348dc2f57c046f8ef63d4538ebb936000f3c9ee954a27460dd865 |
| lifecycle-https.stderr | 0 | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| lifecycle-https.stdout.jsonl | 8270 | 662363ac2a9c455d495e4bb2f0a7d48866e65a15d0654f61671cdfdea43f85fa |
| root_workflow_lifecycle_http_test.go | 3879 | 1b50a59eaac41c98af1ed4c5966ff6df8c8bed17c92f1acb19b346d948ff596c |
| run.json | 7482 | f64209f5af036cf8acc2a25925ae9a0cc99da5cab012dff64f6f953dd548dc90 |
| workflow_lifecycle_action.go | 8316 | 975efcb8d3b5bd49afaeadab075041640630bd20224cbf5ce7e7cc689025a407 |
| workflow_lifecycle_preview.go | 12144 | 948a164015b19a0807b933de77aa4c411bf72ce3d03ed1bb7f91e27a3effa390 |
