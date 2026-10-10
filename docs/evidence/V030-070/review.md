# V030-070 review — complete structure templates

## Scope and sources

- PRD: https://app.notion.com/p/3f52f5a9e64881c8b89fe52110ff3d5e
- ADR: https://app.notion.com/p/3f52f5a9e64881c6a32af33b53bd6ca1
- Data: https://app.notion.com/p/3f52f5a9e648812d915ce9fdd4ac80a3
- Root directly implemented in its own environment. No delegated development, Codex Cloud task, user-computer execution, main merge or deployment.
- Actual dependency PR78 head 913504da had all 22 jobs pass, then squash 9a39b91b was fetched from develop and merged. The two conflicts were resolved by preserving both task entries and recursively merging independent OpenAPI keys. No pre-existing OpenAPI value was changed/deleted in a recursive comparison against actual develop.

## Verified local behavior

1. Exact-key UTF-8 bounded manifest. Complete fields, nested layouts, graph, triggers, groups, grants and all identity references. No business rows, runtime instances, engine payloads, credentials or source owner.
2. Owner/actual Bootstrap export in one read-only RR transaction. Missing workflow candidates, pending deletions and unknown stored configuration fail closed. Single-connection execution and concurrent snapshot behavior verified.
3. Complete explicit typed external mappings; no accidental UUID text substitution. All internal IDs and dependent option/layout/graph/permission references rebuilt, source detached and unchanged.
4. Read-only preflight checks live creation authority and active targets without writing application, personnel configuration, operations or audit. Not reusable authority for later import.
5. Atomic import rechecks current account/actual create permission and source rows, then creates current-owner application, parent-first directories, empty typed physical tables, shared views, groups and disabled unpublished candidate workflows. Existing controlled DDL and catalog compilation are reused. No new runtime privileges or engine call.
6. Dedicated application.template.import fingerprint/closed 201 receipt. Same original key/body recovers after create/target revocation; altered body conflicts; binding order does not change fingerprint. Concurrent same-key import yields one application.
7. Real mid-DDL/metadata and late form/workflow/audit/receipt failures roll back complete structure including physical tables/catalog registration. Actual PostgreSQL COMMIT acknowledgement loss is unconfirmed, then the original durable result recovers without duplicate writes.
8. Real HTTPS import returns 201/Location; invalid/unknown/duplicate/case-confused/oversized input, Session/CSRF/actor and permission failures reject. Unknown commit has only operationId and no successful Location/Cookies. Actual Redis revocation after confirmed commit still returns confirmed 201 and clears Cookies.
9. Export → explicit mapping → import → export retains all supported fixture configuration modulo regenerated identities and unordered resource listing. This is accompanied by independent typed reference/ID remapping and physical-type/default tests, not just roundtrip self-consistency.
10. auth_backup logical dump restored into an isolated empty PostgreSQL database. Seven import receipts and all imported metadata, fields, groups, graph/version/BPMN and physical column definitions compare byte-identically. Six imported workflows remain disabled/current0/candidate1. Restored runtime has no raw physical INSERT, backup has SELECT. Actual Goose nonempty32 Down rejects 55000 and remains32; empty32→31→32 succeeds.

## Evidence / truthful failure history

Each task commit preserves actual order, and docs/evidence retains recoverable RED tests/stubs, source patches, commands, versions, raw results and exit codes. Names such as export-green/import-service-green are labels, not assertions: their failed logs are explicitly retained.

- Manifest/reference/remapping independent RED preceded implementation; export unknown-config/missing-version failures were reproduced before fixing.
- Initial HTTP error checks incorrectly used a success-only helper; corrected before counting valid HTTP RED. Null-bindings fixtures were corrected to isolate null without an unrelated extra key.
- Initial shared Bootstrap fixture cleanup did not handle an existing Bootstrap from other packages; tests now reuse real active database identity without changing the singleton production constraint.
- Initial preflight-era Q36 timed out; exact unchanged budgets passed three isolated runs and the subsequent complete table-stage regression. Original failure remains recorded.
- Initial physical default test compared timezone-formatted text to UTC. Equivalent instant verified; projection explicitly converts to UTC, preserving original expected instant.
- Initial import omitted grant_fields.table_id; existing real database trigger correctly rejected it. Fixed by validated form→table mapping. Replay tests now compare JSON values rather than JSONB whitespace.
- Empty Down test originally shared nonempty imports, then had pending deferred events. Owner-only synthetic transaction establishes the empty precondition and settles constraints; rollback restores rows/schema. No production constraint/trigger disabled.

## Results and remaining gates

- Table-stage 13-package real PostgreSQL/Redis race regression: 905 top-level / 762 subtests passed, three pre-existing capacity skips.
- Full import focused service/schema/fault/roundtrip: 12 top-level / 22 subtests passed; public import HTTP: three top-level / seven subtests passed.
- Current contracts/foundation/governance: 547 passed, zero skip/failure. All Go vet/build passed. OpenAPI 3.2.1 valid with 13 existing warnings.
- Fixed HEAD 5108b46e source archive gitleaks scan: 123.44 MB, zero findings. Production/source diff whitespace checks pass; immutable raw RED logs/patches retain original whitespace.
- First combined 13-package import regression: 948 top-level / 840 subtests passed, one top-level / four subtests failed, three baseline capacity skips. All four failures were a test assuming every application-category catalog entry has a UUID apps row; personnel tests legitimately register external test-A/test-B/test-C entries. The revised assertion compares the entire catalog before/after the injected failure, without deleting existing entries or changing product behavior. The exact retained database had 348 pre-existing external catalog entries after all packages completed; all four revised failure probes passed there without clearing it. Final full 13-package rerun at 9f5000ac: 949 top-level / 844 subtests passed, three baseline capacity skips, no failure. Exact PR #79 head CI, browser/product/runtime/backup/schema gates remain required before develop squash.
- Copy/delete product semantics remain awaiting user confirmation. Saved personal application query presets are outside this template task. This PR does not claim complete delivery of every forms/approval backend feature.
- backwardCompatible=false and no main promotion/deployment authorization. Preserve task source/necessary recovery references until the documented integration/promotion cleanup conditions are met.

## Readiness and final external gate

All local template acceptance is verified; task state ready means reviewable, not accepted or CI-green. The final documentation head must pass its own applicable CI before expected-head squash into develop. The initial PR head 79aa5a46 failed fast task identity because the PR number did not exist at publication; heavy jobs were not run. PR79 is now bound and the same scope check with its verified base/head metadata passes. Final CI verification is recorded on the PR after observing that actual SHA; no claim of future checks passing is embedded in this source. Main and deployment remain separately blocked.
