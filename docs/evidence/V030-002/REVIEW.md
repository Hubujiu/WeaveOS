# B1 evidence and limits

Source authorization: 2026-10-02 PRD 13:14:33.192Z and ADR009 13:14:39.459Z B1
PLAN, subsequently released by root. Both whole-page states remain unapproved;
only the stated backend scope is authorized. Fresh main and read-only PR21 SHAs
are recorded in `docs/tasks/V030-002.md`.

The `*.red.txt` files preserve the actual no-behavior declarations and independent
tests before implementation. Run metadata JSON records UTC start, command, CWD
and process exit status; JSONL contains the original Go output. Logs/fixtures
contain only synthetic references and synthetic invalid payloads.

1. Domain RED: 31 failing test/subtest events, caused by missing rejection or
   partial/mismatched store snapshots. Domain GREEN: 33 passing events.
2. Template RED: 23 failing test/subtest events, caused by missing template
   rejection/validation, absent safe preview state and missing copy isolation.
   First attempt artifacts with `.initial` also retain a harness indexing panic;
   it was replaced with a missing-structure assertion before corrected RED and
   before GREEN implementation. The harness panic is not a valid target RED.
3. Integrated B1 GREEN/race: 56 passing test/subtest events. Module vet and BFF
   build pass. `verification-runs.json` records their exact commands.
4. Benchmark shapes have 1k/10k/50k directory nodes. The 50k chain/star samples
   measured ~11.14ms/~10.83ms and ~7.52MB per validation. This is synthetic
   local-process metadata validation on saved cloud Linux/amd64; it is not a
   database or production performance measurement, depth limit or SLA.
5. Repository structure and existing task checks pass; governance/foundation
   tests pass 132/132, no skips. The existing task checker enumerates V010 files
   only. Direct validation of this V030 task returns `[TASK] invalid id` as
   recorded in `task-governance-gap.json`; a pass of the old checker does not
   establish V030 PR readiness. RED source hashes are in `snapshot-sha256.json`.

Expected traversal is linear in metadata node/reference counts, with hash costs
depending on ID length. There is no unapproved fixed quota or recursive stack
limit in this package. The full template parser needs root-approved byte/depth/
archive/component policies before accepting external files.

Remaining root decisions/integration:

- Stable ID generation/encoding/reuse/import remapping; group cardinality/root/
  order/name/delete semantics and metadata schema/write/CAS.
- Full template manifest/version/bounds/compatibility, component schemas for
  fields/layout/workflows/roles, binding kind registry and target resolver/
  authorization, atomic import/export. The B1 preview intentionally has no
  binding or activation operation; raw components currently fail closed.
- App permission matrix, central entry registration/disable, transactional entry
  recheck and audit integration. Existing `personnel.AllowApplication` and
  trusted Session/identity are reusable; `AuthorizeWrite` is personnel-specific
  and `appendChange` is an unexported personnel-event writer.
- Shared task governance only recognizes V010 IDs/branches. It must be updated
  by root before a V030 PR can satisfy scope validation. B1 changes no shared
  scripts, OpenAPI/proto/migrations/dependencies or PR21 content.

NOT RUN: real metadata PostgreSQL/Redis persistence, DB transaction/concurrency
proof, full manifest/archive/remapping/binding/import/export, HTTP app-internal
authorization, complete module DB/Redis tests, browser/product E2E, backup/
restore or production. A fake Reader verifies the boundary contract and does
not prove persistent atomicity. No merge or deployment was performed.

## 2026-10-02 stacked draft integration update

Draft PR #23 targets B0 PR #22 branch. Ordinary merge `11ce7269…` consumed B0
`5571420c…`, preserved original B1 `72b52e1…` as an ancestor, and changed no B1
application source. The original V010-only checker gap above is historical;
current B0 validates V030 without weakening scope, acceptance or cleanup guards.
Current local checks pass: governance 182/182 with zero skips/cancellations,
B1 race 56 passing test/subtest events, module vet/build, repository structure
and actual PR23 stacked scope validation. Results are recorded in
`integration-runs.json`; final CI is evaluated on the pushed
head. Task metadata records the real PR number and stays blocked for root-owned
product integration, not accepted. Source pages were re-read and the user
explicitly approved integration pushes/draft creation; only documentation is
updated here (TDD:N/A). Main merge, deployment and cleanup remain unauthorized.
