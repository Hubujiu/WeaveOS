# V030-044 ordinary Edit read-only gate

Source: V044 PRD/ADR and P2c fifth-stage contract, FLOW-07/08; source bodies synchronized and reread before code. No new editing exception is inferred. User's 10:10 manual eligibility confirmation is separate and not implemented by this checkpoint.

## Real chronological evidence
- Test-first commit 21159e0: starting/active service and real HTTPS expectations. `ordinary-readonly-red` returned successful version 2 / HTTP 200 instead of the required rejection. No environment failure was counted as RED.
- Initial implementation blocked nonempty ordinary edits before DML. `green-1` deliberately retained a failed old stale-basis fixture: that test used ordinary Edit during a live workflow. Replaced its setup mutation with the actual authorized node Save, preserving all stale-basis and zero approval-side-effect assertions. `green-2`: 30 top / 22 sub PASS.
- `regression-1` preserved a second stale-basis fixture failure in HTTP. Used existing editable task fixture and real node Save HTTP instead of ordinary PATCH; old 409 basis-changed / expired and refreshed-approval assertions remain unchanged.
- Test-first commit 2619d86 added empty PATCH and error-contract coverage. `empty-red` returned a successful receipt despite live workflow. `contract-red` failed because WORKFLOW_RECORD_READ_ONLY was unregistered. Initial uncommitted implementation is preserved in the empty-RED source patch.
- Moved the gate into the ordinary Edit authorization adapter, after existing actual row/CAS/fence/action authorization and before all writes/audit. Empty and nonempty changes are now covered; node Save uses its original separate adapter. `green-3`: 32 top / 22 sub PASS.
- Added independent regression for all four terminal states, another record, confirmed operation replay after its own update trigger starts, same-value rejection, and real row/version/history/audit/receipt remaining unchanged on denial.
- `ordinary-readonly-final`: six packages, 505 top / 226 sub PASS, 0 failures/skips. `ordinary-readonly-java`: all 5 actual HTTPS→Java/Flowable record trigger cases PASS. This local Java run does not claim the three older Docker-only action cases.
- Node contracts/foundation/governance: 513 PASS; Go vet, structure/task checks, source diff formatting, Gitleaks pass. OpenAPI valid with 12 existing warnings.

## Lint safety and evidence handling
The first Redocly invocation printed validation but execution completion was denied because default telemetry could send project metadata. Its log is retained without a success exit claim. Inspected installed 2.54.2 source: REDOCLY_TELEMETRY=off skips sender; REDOCLY_SUPPRESS_UPDATE_NOTICE=true skips update fetch. Checked every OpenAPI $ref is a local fragment. Retried with both supported switches; actual completion exit 0. No telemetry permission was assumed or bypass attempted.

Raw evidence is losslessly archived, with every file hash verified by reading the archive back. Original directories were moved reversibly to the private workspace evidence stash, not deleted. The archive contains failures as well as successful results; a directory name containing `green` does not assert its exit succeeded. Sources are the recorded commit plus the recorded uncommitted patch. Initial RED/contract tests are additionally recoverable from preimplementation-tests.patch after squash.

No migration, role, main merge, deployment, or delegation in this checkpoint. PR69 remains draft/in progress; ordinary manual, rework/resubmit/re-review and other full-backend gaps remain.
