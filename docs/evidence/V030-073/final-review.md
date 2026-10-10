# V030-073 root final local review

Fixed runtime/test source: 7639e169 (full SHA in full-corrected/source-head.txt). No runtime edit after that run. Evidence-only publication does not replace final remote-head CI.

- Sources: task PRD/ADR/data contracts read before RED/implementation; all four object deletion kinds covered.
- No cascade, physical row removal, expiry/purge, deployment or main promotion. Nullable metadata tombstones retain shared typed rows, history, receipts and evidence.
- Current actor/session, manager authority, CAS and finite SQL capabilities; minimal replay before current resource eligibility, unknown COMMIT and renewal failure verified through real TLS host.
- New child/reference writes, public reads, export, workflows and stale deleted layout schema dependencies checked. Six concurrent create/delete interleavings and transactional receipt/event failures covered.
- Full corrected Go: 1017 top-level + 1020 subtests pass, 3 existing opt-in capacity skips. Native Node 581 pass, zero skip. Actual HTTP 7 roots/24 subtests pass; AJV actual responses passed. vet/build/gitleaks pass; OpenAPI valid with 14 warnings.
- Seven Down paths: empty restores exact old bodies/capabilities; marker/event/receipt refuses with unchanged snapshots. Original migrations immutable. Old roles prefix byte-exact; finite new grants only.
- Native restricted backup/restore compares every table in auth/personnel/applications/appdata, including all four deletion event types and ten minimal operations. Docker encrypted archive/recovery and real Flowable suite remain remote CI gates, not local claims.
- Retain genuine RED and intermediate failures. Test-only corrections: pre-existing API route spelling, existing null policy conflict body, synthetic single-Bootstrap slot isolation. Production assertions/unique constraints not weakened. Existing fence SQLSTATE regression fixed in wrapper; explicit fixture FK cleanup list, no CASCADE.

Large raw JSONL logs are losslessly gzip archived after gitleaks passed; full-log-archives.json records original and archive digests/sizes. Example: gzip -dc full-corrected/go.jsonl.gz. Original test commands, runner, environment, source head/patch, exit status and small summaries remain alongside each log.

Pending: exact final remote head CI and root merge review; structure copy is a separate unfinished scope. Local completion is not user acceptance/main approval.
