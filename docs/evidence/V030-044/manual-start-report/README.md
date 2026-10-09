# V030-044 ordinary manual start and recovery-fixture correction

## Source / bounded outcome
User Sentinel_eb82c0933ff48191a7766fa19997982a at 2026-10-09 10:10 UTC approved the previously asked creator-only first manual start. V044 PRD/ADR and P2c bodies were updated and reread before implementation. Existing flow history requires dedicated original-initiator resubmission or original-graph approver re-review. Current ordinary API neither implements nor bypasses those later actions.

POST .../records/{recordId}/workflow-starts accepts a closed creator-scoped request, checks live session/menu/row access and CAS, evaluates the current published manual trigger on the actual typed record, and commits a starting intent plus minimal operation receipt atomically. In-flight same-flow duplicates are ignored; terminal history is rejected; A/B independent. No business record writes, RPC in the transaction, new grants or fabricated command ID. Accepted is durable intent only, not engine completion.

Migration25 extends only the operation kind and tightly constrained 202 result. Old migrations/roles unchanged. Down refuses stored manual receipts. Exact hash registration is tested. Routine replay remains actor-scoped minimum recovery; conflicting identity cannot replace the result.

## Actual test sequence
- e7f115d: declaration-only service stub + independent service tests. `manual-start-red`: 8 top/13 sub failures for absent behavior after real fixtures succeeded.
- 718aaa6: actual PostgreSQL result constraint tests before migration. `manual-receipt-red`: both valid receipt inserts rejected by missing operation kind, rather than a missing-file/setup failure.
- 1efaba1: migration registration test before registration; actual missing-entry RED.
- Service/catalog/migration implementation then `manual-start-green-1`: 10 top/13 sub PASS.
- 999a6b2 includes the already-tested catalog/migration implementation and newly written HTTPS tests. It is not a pure test-only snapshot despite its concise commit subject. `manual-http-red`: 2 real HTTPS tests failed with missing-route 404; four ingress security subcases already passed.
- 823ed58: closed OpenAPI tests. Both missing route/schema RED before contract addition.
- HTTP/decode/schema wiring then `manual-http-green-1`: service10 + HTTP2 PASS, 17 sub PASS. `manual-contract-green`: contract/registration PASS.
- `manual-java-regression`: seven real public HTTPS→Java/Flowable scenarios PASS (five automatic + manual start and actual committed reply-loss recovery). This is end-to-end regression after the public HTTP RED, not an invented earlier RED.
- 74357af: CI identity/selection tests before gate/runner change; actual 1 failure + 1 error. Strict gate now requires original3 action scenarios + automatic5 + manual2, plus agree/reject sub-identities: 12 exact identities. All 11 gate unit tests PASS; timeout/skip/failure rules unchanged.
- Extra authority/rollback/cross-view/replay regressions retained. First cross-view fixture incorrectly tried changing immutable application ownership; fixed by installing real menu/read grants on another view, without weakening the database rule. Next full run exposed raw pgx.ErrNoRows for a foreign flow instead of the domain ErrMissing; corrected that specific boundary after the real failure.

## CI failure found and corrected, not hidden
Exact remote79d9e2f workflow-recovery job113777135133 failed in the older LifecycleInterop test. Downloaded artifact11610726336 ZIP hash2404e4679bf42512b284335b9da8f594928f8936b52ec59a9bf5e431fa9ccef8 proves ordinary Edit on an active instance was rejected by the new correct read-only rule. Other two recovery cases passed.

Added a strictly authenticated native loopback test transport alongside the unchanged dedicated Docker host. `readonly-recovery-native-red` reproduced the original Edit failure against actual Java. Changed only the concurrent-write fixture to a genuine current node Save with complete field definitions. Kept original start + three decisions, task counts, exact receipts, projection and historical record versions [1,2,2,2] assertions. `readonly-recovery-native-green`: all three genuine recovery cases PASS. Production read-only rule unchanged.

## Final local checks
- `manual-final-regression-2`: seven actual PostgreSQL/Redis Go race packages, 534 top / 271 sub PASS, zero failures/skips.
- `manual-and-recovery-final`: three genuine recovery + seven public automatic/manual Java scenarios PASS. These ten selected native tests are not the CI action job's ten scenarios: the latter includes three older action tests and remains to be confirmed remotely.
- Node contracts/foundation/governance516 PASS; Python strict gate11 PASS; Go vet, exact scope/format/structure, Gitleaks pass. Redocly with telemetry/update checks disabled: OpenAPI valid, 12 existing warnings.
- No Docker locally. No production deployment, role changes, main/develop merge, user computer or delegation.

Raw archives retain all failures, source heads/patches, commands, timestamps and actual exit codes. Every archived file was SHA256-verified by reading it back. Raw directories were moved reversibly to the private workspace stash, not deleted. `initial-service-and-receipt-tests.patch` preserves preimplementation declarations/oracles after squash; other per-run patches plus ordered source commits preserve later phases.

Remaining: safe ordinary-user discovery of available manual flows, dedicated rework/resubmit/re-review, deletion/independent history/inbox and other full-backend gaps. PR remains draft/in_progress. This checkpoint is not full backend completion.
