# V030-064 local stage review — 2026-10-09 15:07 UTC

## Independent scope and original proof
- Three actual initial REDs: missing journal columns, absent exact name/provenance DTO, instance FK preventing projection-only cleanup.
- After minimal DDL, independent actual failures observed separately: detail becomes missing, list silently empty, original terminal replay conflicts. Those sources/logs and exact initial DDL retained before read/replay implementation.
- First DDL syntax failure is not counted as TDD RED. CASE expression corrected with no weakened expectation. Historical RR test locator changed from old JOIN suffix to same event-table read boundary; real revocation/corruption and all assertions remain.
- New 14-field DTO and migration manifest tests failed before contract/manifest changes. Existing 12-field assertions intentionally extended only for the two source-approved fields.
- Index existence RED and measured Seq Scan + top-N baseline preceded index implementation. Small isolated 201 target / 401 noise fixture: 38 shared blocks / 0.203ms before, 5 blocks / 0.027ms after, Limit -> Index Only Scan with 21 heap fetches. This is not production performance or a complete public SELECT benchmark.

## Code/data review
- Original append-only journal stores minimal scope/version/task/node/name, never a second BPMN/graph. Original bytes/hash/proof/time unchanged in tested backfill. Old names explicitly legacy_last_known, new confirmation names captured.
- SECURITY INVOKER, pg_catalog search_path, qualified objects, no public function execution grant or new runtime write authority. Original 13-column projection INSERT remains usable inside the original transaction. Pending ledger allowed at callback; backfill requires terminal receipt identity. Wrong existing task binding rejects, absent no_effect task keeps stable ID and null source node.
- Current record/history authority precedes any event read. Detail keeps original manifest visibility/field integrity. List retains canonical batch ledger verification, fixed expiry/domain/session binding and no full evidence loads. All original API fields remain closed plus explicit name/provenance.
- Terminal-only replay revalidates original command/receipt and exact event bytes plus stable journal binding. Pending/missing takes original execution path, not historical execution. Existing application/table/command lock ordering and close finalization remain. New command fence preservation and changed receipt/scope rejection verified; no new events or reconstructed instances.
- Names/node identifiers are database-maintained immutable metadata, not a newly invented cryptographic signature. Original protocol cryptographic validation remains in Go; SQL migration binding checks are not advertised as full Go codec revalidation.
- Publication ledger/engine receipt catalog FKs remain intact. This stage only cleans isolated terminal task/instance fixtures and changes current configuration. Full catalog deletion, publication replay across deletion, engine deletion RPC and product delete flow remain future work.

## Local results and limitations
- Full related 10-package real PG18.6/Redis8.2.10/race run at b80c58b: 723 top-level / 548 subtests passed, 0 failed; 3 existing optional appquery capacity skips explicitly not passes.
- Two additional no_effect/atomic-failure tests passed after that full run; no implementation changed. Other added post-implementation boundary/migration tests are coverage, not retroactive RED.
- Node governance/foundation/contracts 529 passed; OpenAPI 0 errors / 13 existing warnings; go vet/build passed. Gitleaks scanned 97.90MB with no leaks before the final two test-only additions.
- Real pg_dump as auth_backup and pg_restore into a new isolated database retained exact bytes for 607 captured journal rows, trigger/index and SELECT/INSERT/no UPDATE/no DELETE runtime capabilities. Legacy backfill separately tested; this does not replace CI encrypted recovery validation.
- First tampering fixture wrote the same record ID because foreign=true aliases ownRecord to otherRecord; fixed to a genuinely different UUID without changing rejection expectation. Failure cleanup was interrupted after omitted rollback delayed pool close; both transactions now defer rollback. Original partial output retained and never called a pass.
- Final upstream develop integration, final exact-head CI and remote PR remain pending. No main or deployment changes.
