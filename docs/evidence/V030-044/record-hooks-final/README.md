# V030-044 actual record trigger entry wiring — 2026-10-09

## Existing authoritative scope
V044 PRD/ADR already specify Create/Edit + every matching starting reservation in one application database transaction; FLOW-01/03/05/07/11/12 and the user's actual-change/table-scope decisions supply the oracles. No new business rule, DDL, worker, role or external engine call is introduced here. A proposed Notion implementation-plan appendix was denied and was not retried; that documentation action remains pending user authorization. Re-reading the existing accepted scope established that this minimal existing-contract wiring could proceed independently. Future acceptance-worker decisions are not represented as synchronized.

## Test chronology
Before implementation: c4879f6, 8fedf9a, 2f4e58a; reconstruct test snapshot from record-hooks-confirmed-red/test-source.patch against 113579e0. Actual RED: eight top-level cases failed because valid saves retained no required starting instances, and deferred trigger failure never ran. Six no-trigger subcases already passed; no claim that every assertion was RED. The initial uppercase member-ID fixture was invalid under the existing canonical ID contract and was corrected before implementation, with the original log retained.

Supplemental concurrency, node-Save exclusion, denied writes, lowest valid UUID and HTTPS regressions were added during review; they are not advertised as original test-first evidence. A zero UUID probe correctly failed existing BPMN compilation and was corrected to the smallest nonzero valid ID without any production code change; its invalid fixture evidence is retained.

## Implementation and verified behavior
- Catalog uses current published configuration across the whole table, parameterized existing condition compiler and actual typed records. Disabled/closing/different event/nonmatching condition excluded; candidate edits do not change current matching. Constant result memory, stable flow-ID traversal; existing atomic duplicate reservation reused.
- Create reserves all matching starting instances before operation commit. Edit's DML wrapper runs after existing locks, CAS, fence and field/row authorization; real PostgreSQL typed IS DISTINCT FROM detects meaningful changes. Saves retain existing version/audit behavior even when no trigger fires.
- Real PostgreSQL deferred commit failure proves record, both intents, audit and operation result roll back together. Replay/concurrent replay does not duplicate the intent set. Node Save remains isolated. No RPC is performed inside these transactions.
- Final four-package Go race regression: 392 top-level / 197 subtests pass, zero fail/skip. Dedicated real HTTPS: two top-level tests pass, with real Session/CSRF, definition configuration, record create/replay and same-value/changed edit. Only the deployment receipt in these HTTPS fixtures is synthetic; this is not full Flowable start E2E.
- Node contracts/foundation/governance: 512 pass, zero fail/skip; go vet and Gitleaks pass; task structure and diff checks pass.

## Boundary
Public record saves now durably retain matching starts. Actual start-command admission/background dispatch, public manual entry, rework/re-review and complete backend delivery remain unfinished. No merge or deployment is authorized by this checkpoint. Prior remote CI at 113579e0 was all successful with five existing WebKit animation skips; it does not verify these later source changes.

Formatting clarification: the source/task-only committed diff passes git diff --check. An all-artifact cached diff check flags literal whitespace in preserved raw patch context lines and shell command captures; those are original evidence bytes, not source formatting defects. They were not rewritten to conceal the observation.
