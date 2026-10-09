# V030-063 Root review at local stage

2026-10-09 14:17 UTC. Main assistant; no delegation. Source 226d910 (full SHA recorded in regression-full/source-head.txt); internal V062 parent0028af0, actual develop integration still pending.

- One common current record-history authorization helper for detail/list; real live identity, real resource, read/history field intersection, owner historical exception and missing/denied distinctions retained. Refactor rechecked all13 original detail service and2 HTTPS tests, plus full related packages.
- Each page uses read-only RR and ten-second budget. Candidate query scopes actual app/table/record before LIMIT. Deterministic confirmation-time/UUID keyset handles ties; no total scans or per-event full-basis call in application code.
- Batch ledger uses the original shared hash/receipt decoder, max101 canonical unique commandIDs, no partial map on missing/corrupt entry. Bounded selected events and original historical tasks are checked against original scope/versions/actor/action/outcome/sequence/proof. No pending command masquerades as event.
- List checks original metadata; deliberately does not claim full payload/result/basis byte integrity without loading those bytes. Detail still owns that stronger validation. No fields, hashes, graph, raw protocol or engine IDs in list DTO.
- Opaque token uses existing isolated querycontext namespace and Session LRU, plus actor/resource/pageSize/policy/schema and explicit ten-minute expiry. Rechecking live permission precedes token use. Redis cursor write failure returns no successful partial page. No new security credentials or permission changes.
- Real tests prove current RR versus next-request revocation/corruption, cross-flow/pending, terminal withdrawal/return metadata, same-time cursor pages, new events, fixed expiry and namespace isolation. Post-implementation coverage is honestly marked as such, not retroactively called initial RED.
- SQL count at page1/20/100 stayed11 including tx over101 actual confirmed events. DB may still examine/sort the requested record's accumulated history through existing indexes; this sample does not establish production-scale throughput or justify an unmeasured new index.
- Actual HTTPS and closed OpenAPI validated. Existing QUERY_CONTEXT code spelling and one FK fixture deletion-order error are documented; no correct business expectation or production constraint weakened.

Full related10-package real PG/Redis/race regression: 693 top-level/474 subtests pass, 0 failure,3 existing opt-in capacity tests skipped. Node514 pass, OpenAPI0errors/13existingwarnings, vet/build pass. Final exact remote CI and final actual-develop integration have not run; cannot merge or claim whole backend complete.
