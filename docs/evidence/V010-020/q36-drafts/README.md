Q36 draft backend: in progress, not root acceptance.

Base SHA: 1468a969d25d0ba73586db65e83ac21ea8647b20; independent task/V010-020-drafts-backend.
Approved oracle: docs/tasks/V010-020-PLAN.md §5; contracts/query-drafts.md; OpenAPI draft schemas.
Notion PRD 3ec2f5a9e648814e8e5cf89bb46c068d and ADR007 3ec2f5a9e64881c19aa8d5fc6d821ab8 read back 2026-10-01; updated 12:34:16.119Z / 12:31:05.697Z, approvals through current prose agree with PLAN. Native verification is unverified; tool did not report truncation/unknown blocks.
No .agents/skills files were present in the selected workspace; root and BFF AGENTS read.

RED command: `python3 docs/evidence/V010-020/q36-drafts/run.py test test -race -count=1 -run TestDraft -v ./internal/personnel`.
Actual exit 1, red-valid.log: 10 top-level tests reach missing behavior assertions against compile-only ErrNotImplemented placeholders. RED source preserved as *.go.txt and SHA256 manifest.
First run red.log also exposed an invalid department fixture (missing required parent). Fixed fixture against existing schema, then reran before implementation; only red-valid.log counts as complete RED.
Initialization first failed during PG temporary-server transition (environment.log); retry Up successful (environment-retry.log). Neither is behavior RED. Runner now waits for TCP readiness.
Test-only grants on isolated PG: SELECT/INSERT/DELETE drafts and UPDATE(payload_json,draft_version,updated_at). Production role grants, shared routes/constructor, formal write cleanup call sites remain integrator-owned.
No migration Down or production access. Effective model metadata is unknown: no tool supplied it.

Implemented delegated scope; integration/acceptance pending. Shared files unchanged.

Callable entrypoints (all in package personnel):

```go
func (a *Application) CreateDraft(ctx context.Context, p session.Principal, in DraftCreateInput) (PersonnelDraft, error)
func (a *Application) ListDrafts(ctx context.Context, p session.Principal) (PersonnelDraftList, error)
func (a *Application) GetDraft(ctx context.Context, p session.Principal, id string) (PersonnelDraft, error)
func (a *Application) UpdateDraft(ctx context.Context, p session.Principal, id string, in DraftUpdateInput) (PersonnelDraft, error)
func (a *Application) DeleteDraft(ctx context.Context, p session.Principal, id string, version int64) error
func (s *Service) DraftHTTP(w http.ResponseWriter, r *http.Request, p session.Principal) bool
func CleanupDraft(ctx context.Context, tx pgx.Tx, p session.Principal, ref *DraftReference, kind DraftKind, target *string) (bool, error)
```

`DraftHTTP` runs after the existing shared Prepare/Authenticate chain, including write CSRF. It returns false for other paths. No constructor/route changes or runtime SQL grants were made. Suggested zero-context patches are integration.patch (`git apply --unidiff-zero --check` verified against the frozen baseline; integrator must adapt to current shared files); their shared-file edits are intentionally unapplied.
Application methods reuse existing live `read` / `write` / `AuthorizeWrite` account/auth_version/grant dependency protection. That existing write authorization can create the internal version=0 member_configuration row; drafts themselves write only personnel.drafts and do not append business audit or touch query_revisions. Query context is not consumed or renewed by drafts.

Cleanup integration: call after successful actual business mutation/audit, before commit, in the same authorized transaction. Derive kind and target from the actual operation. For new department/identity/template pass nil target, matching the original draft (not the newly generated business ID); for existing targets pass the actual target. A nil ref or missing/wrong owner/version/kind/target is a no-op returning false. Malformed reference input returns ErrInvalid and caller must roll back. Errors must roll back the business transaction; never invoke after a failed/ambiguous commit, never start a separate cleanup transaction or replay business writes. Concurrently newer saved revisions survive. Shared formal write signatures/DTO parsing and every actual call site are integrator-owned, not implemented here.

Payload encoding: allowlisted explicit fields; strings/array order preserved; deterministic sorted object keys, compact UTF-8 JSON, HTML escaping disabled, required JSON escapes retained. Invalid UTF-8, unpaired surrogate escapes, NUL unsupported by the frozen PostgreSQL jsonb shape constraint, duplicate/unknown/missing fields, wrong variants, invalid UUIDs/enums/types/lengths and >65536 canonical bytes return ErrInvalid. No existence checks on target/reference IDs or name nonempty validation: incomplete/stale inputs must persist. PUT cannot change original baseVersion/kind/target. Draft versions stay within JS safe integers; exhaustion returns draft conflict rather than overflow.
OpenAPI uniqueItems compares JSON strings: a supplemental real RED caught case-folding being too restrictive for case-distinct UUID spellings. red-uuid-spelling.log / red-uuid-*.go.txt preserve the failing test and implementation; the correction preserves valid case-distinct original strings. This is draft input preservation, separate from business UUID relation normalization.

Actual results (2026-10-01 UTC; Go1.27.1, PG18.6, Redis8.2.10):

| Command | Result / evidence |
| --- | --- |
| `run.py setup` | First attempt exit 1 during temporary PG server transition; retry Up exit 0 through migration 00003. Runner fixed to wait for TCP server. environment.log / environment-retry.log |
| `run.py test test -race -count=1 -run TestDraft -v ./internal/personnel` | RED exit 1 (10 top-level missing-behavior tests); first test fixture error corrected before red-valid.log. Initial GREEN exit 0 in green-initial.log |
| `run.py test test -race -count=1 -run '^TestDraftQ36UniqueItems' -v ./internal/personnel` | Additional real RED exit 1 for overrestrictive case-folding, red-uuid-spelling.log |
| `run.py test test -race -p 1 -count=1 -v ./...` | FAIL exit 1, go-all-race.log. Eight existing audit tests cannot TRUNCATE auth.users without including new personnel.drafts FK; existing audit/persistence role reset removes test-only drafts grants, so personnel drafts fail 42501. Other packages passed. No owned-file workaround or weakening of tests |
| Isolated `GRANT SELECT, INSERT, DELETE drafts` + `UPDATE(payload_json,draft_version,updated_at)` | Restored after role-reset tests; test-grants-restored.log. Only dedicated container/database, no runtime file edit |
| `run.py test test -race -count=1 -v ./internal/personnel` | Final PASS exit 0, 43 top-level tests including 14 draft tests, personnel-final.log |
| `run.py test vet ./...` | Final PASS exit 0, vet-final.log |
| `run.py test build ./...` | Final PASS exit 0, build-final.log |
| `node --test tests/governance/*.test.mjs tests/foundation/*.test.mjs` | PASS exit 0, 132/132, governance.log |
| `git diff --check` | PASS |

Real PG checks: 40 simultaneous creates => exactly 20 successes / 20 domain-limit errors; two same-version concurrent updates => one success / one conflict; a slot can be reused after CAS deletion. Foreign owner CRUD returns ErrMissing and foreign list is empty. Revoked grants/disabled user/stale auth_version block every operation. Revoked real Redis Session blocks HTTP mutations and reads; fresh real Session restores the same account draft. UTF-8 exact 65536 accepted, >65536 rejected after canonicalization; raw escaped/whitespace input may be larger. Secret/extra fields and immutable HTTP envelope fields rejected. Summary order explicitly checks updated_at DESC / id DESC tie-break. Target deletion and invalid references preserve inputs and original base. Draft create/update/read do not change stored query_revisions or auth audit. Cleanup rollback restores deletion; exact success commits deletion; wrong owner/version/kind/target preserves; real blocked cleanup rechecks and retains a newly committed version. In-flight save holds actual authorization dependency locks.

Not run: actual shared route registration, formal business write call-site cleanup, integrated later revision-trigger migration, frontend/screenshots, full product/browser/runtime/restore matrix, security scanner and final PR21 CI. These belong to integrator/frontend/final acceptance; broad Go failure is unresolved here. No migration Down, production deletes, merge, deployment, force push or replacement PR. Test runner cleanup removes only its labeled dedicated containers/volumes and private generated connection files; worktree and branch remain for integration.

Stage handoff requested by root after safe checkpoint; retain dedicated containers/private fixture and worktree, do not roll back. Root 2026-10-01 steering changes table query to POST search; drafts are independent. At 7e03b6 the inner delegated decoder still had the existing 1MiB fallback; the root follow-up below freezes and implements the independent 128KiB decoder limit. Shared route registration remains integrator-owned. No table query contract files were changed.


Root-requested follow-up: draft decoder rawbody cap (2026-10-01 UTC).

- `draftDecodeBody` now directly enforces **131072 raw UTF-8 bytes (128KiB)** for the entire POST/PUT body: envelope, source JSON escape spellings and whitespace count. `canonicalDraftPayload` separately caps **65536 canonical UTF-8 bytes** of payload only. The transport limit does not replace or relax the storage limit.
- New real PG/Redis tests: `TestDraftHTTPQ36RawBodyLimitBoundary` accepts exactly 128KiB for both create/update, rejects 128KiB+1 with COMMON_INVALID_ARGUMENT, and verifies no new row/version/payload mutation on rejection. `TestDraftHTTPQ36CanonicalPayloadWithWrapper` accepts 65536 canonical payload bytes plus the envelope for create/update, then rejects 65537 payload bytes while the raw request remains below 128KiB.
- Actual RED command: `run.py test test -race -count=1 -run '^TestDraftHTTPQ36(RawBodyLimitBoundary|CanonicalPayloadWithWrapper)$' -v ./internal/personnel`, exit 1. rawbody-red.log shows create/update wrongly accepted 128KiB+1 and committed changes. The 64KiB payload+wrapper control passed before the fix. Pre-fix implementation/test sources are rawbody-red-*.go.txt, with rawbody-red.sha256; RED was observed before the decoder change.
- Actual final GREEN command: `run.py test test -race -count=1 -v ./internal/personnel`, exit 0, **45 top-level tests including 16 draft tests**, rawbody-personnel-green.log. `run.py test vet ./...` / `run.py test build ./...` each exit 0, rawbody-vet.log / rawbody-build.log. Existing quota/CAS/owner/CleanupDraft functions unchanged; no shared router/roles/TRUNCATE/DTO/schema/query-contract file changed.
- Existing `TestDraftHTTPQ36SessionCSRFAndRestore` exercises real authenticated POST create, PUT update and versioned DELETE **without queryVersion/context**. `TestDraftQ36PreserveTargetBaseAndNoBusinessEffects` now explicitly begins with no authorization configuration row, verifies first draft save creates only `member_configuration.version=0`, and compares stored revision values before/after initial save and draft-only update/delete. The genuine target deletion is rebaselined separately so a later revision trigger can legitimately record that business deletion; assertions on draft-only effects remain unchanged.
- **Pending integrated query/revision verification:** this frozen branch contains migration 00003 only, without the later revision-trigger/queryguard implementation. The tests above prove current no-queryVersion CRUD and baseline revision nonmutation, including initial version=0 authorization storage. They do not claim a real query context was expired or that the final revision-trigger exception has passed. Once the integrator installs queryguard and revision triggers, rerun these draft tests plus an actual expired-query-context/initial-authorization-row schedule; Create/Update/Delete must still succeed when Session/permission are valid and must not be forced through queryVersion validation merely because they reuse `a.write`. No mock query state or fake trigger was added to claim that proof.

Shared roles/TRUNCATE/router blockers from go-all-race.log remain delegated to the unique integrator. Test containers/private fixture and clean worktree are retained for root continuation. No security scanner, merge, deployment, migration Down or production delete was run.
