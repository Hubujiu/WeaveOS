# V030-043 first scoped implementation and validation

Root released exactly four production paths after personally reading strengthened RED at89d9a4c. Implementation commit `fa7d05b5c844fc11f78bab161022bc72ddc26843` has parent `89d9a4c955ac8b8bc15a0cc3f9c7557b6296ee16` and changes only workflow_save.go, workflow_preview.go, apprecordhttp/workflow.go and appstructure/record_http_decode.go. The pre-run production.patch and file hashes preserve the exact tested bytes; no source changed during/after this first test run.

Actual new PRD/ADR were fetched read-only before implementation: PRD3f22f5a9e64881159058fa3869005e1b, last-edited2026-10-07T02:06:38.874Z; ADR3f22f5a9e6488103abfcf6a44eceb773, last-edited2026-10-07T02:15:49.568Z. Both identify this as a Root execution contract and retain unverified/global-release status; no source approval state changed.

## Implementation
Save uses existing RecordWrite and Writer.EditInTx in one business write transaction. Request fingerprint includes workflow.task.save plus the complete request; persisted kind remains record.edit. Confirmed replay uses the existing GetRecord read path to verify this actual row remains readable, while retaining the original write lifecycle locks, and bypasses expired basis/task-lifecycle checks. It introduces no second business write.
New acceptance checks current identity/menu/actual row visibility, then table gate, pinned instance/task locks and actual record lock. Reuses current evidence hash/version checks, pending fence, pinned node whitelist, actual-createdBy CanEdit, normalization and active reference validation. A local controlledDML adapter overrides only UpdateCAS to use RecordHistoryDML with task_save and immutable server JSON task identity. Complete200/Commit precede success; original unknown-COMMIT behavior remains. No RuntimeReady probe, engine command/dispatch, new fence or engine change.
Preview returns a stable sorted non-null editableFieldIds intersection with actual visible/editable fields and the pinned node. PATCH reuses existing session/CSRF/Origin/expected-actor and closed-body decoder; only the new kind gets nonempty Changes validation, with the established1MiB body limit and existing typed-value/duplicate-key validation.

## First verification (no iterative debug or test changes)
- `go test -race -p 1 -count=1 -json -run '^TestRootWorkflowSave' ./internal/apprecordservice`: exit0,18top-levelPASS and2basis-subcasePASS,0FAIL/0SKIP. Includes strengthened trigger sequence fault, actual rollback/lost COMMIT, concurrency, task/field authority and no engine dependence.
- `go test -race -p 1 -count=1 -json -run '^TestRootWorkflow(SaveHTTP|ActionHTTP)' ./cmd/bff`: exit0,8top-levelPASS and26subcasePASS,0FAIL/0SKIP. Root's old Action response exact-five-key/read-only-empty-array assertions remain unchanged. Real local HTTPS listeners succeeded.
- Node registration contract3PASS, exit0; registration alone is not HTTP proof.
- `gofmt -l` on the four production files: exit0, empty output.
- `go vet ./internal/apprecordservice ./internal/apprecordhttp ./internal/appstructure ./cmd/bff`: exit0, empty output.
- `git diff --check`: PASS. No full-repository race/build/product/formal-engine acceptance run was performed; Root requested scoped first-version review.

All Go stderr streams are empty and no race report appears. Original stdout JSONL, stderr and exits are retained byte-for-byte after credential/DSN/private-path review. Public metadata replaces private socket/cache identities with descriptive values; original complete run metadata remains in the task-private directory. Hashes and per-case results are in run.json. Both earlier environment/RED histories remain untouched.

Root test/fixture/OpenAPI/dependency/migration/role/engine/CI files are frozen and unchanged. Test hashes match a6859b7(service),746bd631(SaveHTTP),ea4b2826(ActionHTTP); full hashes and locked go.mod/go.sum hashes are in run.json. The original declaration-only stub remains preserved in original-test-seam with hash380f14ca.

Task metadata still lists appstructure/record_http.go rather than record_http_decode.go; this implementation follows Root's explicit four-file release. Root should align allowedPaths before final scope/CI review. This executor appends results only, without altering the Root-owned task contract.

Pending Root source review, debugging/expanded acceptance and final checks. No PR, merge, main or deployment action. Original task-only resources retained for Root's next authorized run.
