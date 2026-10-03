# V030-015 consumer review follow-up · 2026-10-03

This checkpoint extends isolated branch `task/V030-015-records-query-drafts`
after `24d7be1`. It changes only V015-owned `apprecordservice`, `appdrafts`,
task documentation and evidence. It does not change V013-owned HTTP,
OpenAPI, migrations, roles or controlled DML.

| Review finding | Correction and real PostgreSQL/Redis evidence |
| --- | --- |
| A stale schema/base draft could not explicitly remove an old key | `UpdateDraft` permits only a nonempty `removeFieldIds` with empty `changes` across the stale binding. `Store.UpdateInTx` requires every removed key to exist in this Session owner's locked draft; schema/base bindings remain unchanged and the response retains conflicts without old values. New values still fail. Physical field drop, simultaneous schema/base conflict, revoked grant and same-transaction operation/audit checks are in [RED](consumer-draft-conflict-cleanup-red.txt) and [GREEN](consumer-draft-conflict-cleanup-green.txt). |
| Omitted member/department defaults bypassed active-source checks | Create now checks the effective reference from an explicit value or current schema default under `BeginRecordWrite`'s source revision lock. The physical DB default remains server-applied; a caller needs no create mask on the defaulted field. Active default, disabled default with zero new row/audit/confirmed operation, and preservation of an unchanged old reference are in [RED](consumer-default-reference-red.txt) and [GREEN](consumer-default-reference-green.txt). [Race evidence](consumer-default-source-race.txt) observes the create blocked on the source writer's revision lock, then rejecting after that writer commits disabled status. |
| A removed saved quick-search field returned invalid/forbidden instead of query changed | Saved criteria check current field existence before checking its current grant. Missing/kind-changed saved fields return `querycontext.ErrChanged`; a retained field with revoked read permission remains forbidden; invalid incoming criteria remain invalid. [RED](consumer-quick-schema-red.txt), [GREEN](consumer-quick-schema-green.txt). |
| Redis Advance failed after the RR receipt committed, preventing original-key recovery | Both RR validation and post-RR Redis Advance failures now take a **replay-only** V013 live-Session/actor/key/kind/fingerprint path. A confirmed operation returns its minimum result; an absent operation returns the original read error. No Claim or business DML occurs in this fallback. Confirmed, unconfirmed and inactive-Session cases are in [RED](consumer-advance-replay-red.txt) and [GREEN](consumer-advance-replay-green.txt). |
| Create conflated authorized schema-not-ready with denial | Live menu and create grant checks run first. Authorized not-ready returns `APPLICATION_SCHEMA_NOT_READY`; unauthorized access remains denied. [RED](consumer-notready-red.txt), [GREEN](consumer-notready-green.txt). |
| The service record DTO omitted frozen identity/control fields | Both page items and detail now return `appId`, `tableId`, `viewId` and same-RR `schemaVersion`. The page also returns `page`, `pageSize` and canonical `sort`. The full-P fingerprint still excludes schema control versions. [RED](consumer-record-dto-red.txt), [GREEN](consumer-record-dto-green.txt). |
| Reference `EXISTS` in the predicate hid missing registry rows before integrity checking | Reference UUID equality now evaluates on the typed column. A separate authorization/mask-conditioned integrity check runs before Observe COUNT, and page hydration fails if a visible page reference has no source row. A retained tombstone still matches and displays `deleted=true`. [RED](consumer-reference-integrity-red.txt), [GREEN](consumer-reference-integrity-green.txt). |

The V013 Save port updates `logical_tables.fields_json` and physical column
defaults in one table-gated transaction. V015 checks those trusted metadata
defaults before controlled DML while the PR21 personnel/source revision locks
are held. For defense in depth, V013 can assert metadata/physical default
parity in its controlled `RecordDML.Insert` port or expose the effective
default reference set from the same gated transaction; V015 did not change
that shared port. It must not require the caller to send defaults or use the
caller field mask as a substitute for source validation.

The combined [V015 core race suite](consumer-followup-core-race.txt) passed
against PostgreSQL 18.6 with runtime `SET ROLE auth_app` and Redis. The
[BFF-wide follow-up](consumer-followup-bff-race-shared-skip.txt) retains one
explicit skip for the V013-owned B5 disabled-member test. Its old oracle
expects acceptance of a newly inactive member, while the frozen newer policy
requires active members; V013 owns that oracle repair. A skip is not a fully
green BFF gate.

Shared HTTP/OpenAPI/config/history wiring, real Session/CSRF and browser API
acceptance, an injected unknown-business-COMMIT service test, and the broad
million-row × 200-context full-chain capacity gate remain open. No compact
algorithm or timeout substitution was made.
