# V030-015 shared integration delta for V030-013 owner

Status: **reviewable proposal, not migrated or available through HTTP**.
Authoritative behavior is the [frozen V030-015 ADR §8](https://app.notion.com/p/3ee2f5a9e6488163a144fba5f55e3c14).
This delta was checked against V030-013 remote `4dcad40f09d5d20c2d2d9f97a12c7e41b4630bed`
on 2026-10-03. V015 worktree remains based on the fixed PR26 head and
does not modify V013-owned migration, role, OpenAPI, error, policy, personnel,
application transaction or BFF files.

## 1. Current exact shared facts

- V013 hot7 creates `applications.logical_tables` with `schema_version`,
  `schema_ready`, `data_revision`, `dependency_revision`, `fields_json`, and
  `applications.form_views` with `view_version`. It creates typed
  `appdata.t_<table UUID without dashes>` with system columns
  `id`, `created_by`, `created_at`, `updated_at`, `record_version`.
  `applications.data_revision_changed` fires after each INSERT/UPDATE/DELETE.
- The dynamic table creation arm in `applications.apply_schema_change`
  grants **SELECT only** to `auth_app`/`auth_backup`. `auth_app` has no
  INSERT/UPDATE on business tables. V015's current `apprecords.Writer` uses
  direct typed `INSERT`/`UPDATE` under caller-owned `pgx.Tx`; it passes local
  PG owner fixture tests but cannot execute in the reviewed runtime role.
- `appfields.NormalizeValue` normalizes complete values and defaults,
  including exact decimal rounding, UTC time precision and option ordering.
  It does not itself verify a member/department is active in an authoritative
  reference source. Draft incomplete input deliberately needs a separate
  shape/length path.
- `applications.BeginManagerWrite` requires owner/Bootstrap after
  `personnel.lock_query_revisions()` and `AccessForWrite`; ordinary data
  writers must not call it. `applications.operations` still allows only
  manager/definition kinds, status 200/201; its read route uses
  `readableOperation`, which requires manager for all except app creation.
- B5 grant SQL and `applications/store.go` still accept menu only. Hot7
  registers real form menu resources but has no persistent data grants or
  field-mask validation. The existing `apppolicy` evaluator has no
  `data.create` action-level check.
- Q36 Redis query context, lifecycle and write receipt live in package-private
  `services/bff/internal/personnel/query_context.go`, `query_engine.go` and
  `query_write_guard.go`; the metadata validator admits only `members/events`.
  V015 cannot call it as a records context without a coordinated extraction.

## 2. Ordinary record write lifecycle and ports

Provide a **non-manager** `BeginRecordWrite` alongside V013's existing
`BeginManagerWrite`, with equivalent `Tx`, `Replay`, `Claim`, `Complete`,
`Commit`, `Rollback` and unknown-COMMIT classification. Suggested trusted
input: Session principal, actual app/view/table IDs resolved server-side,
operation kind and SHA-256 fingerprint, optional queryVersion, configured
lock/statement limits, source-lock callback. Never accept an actor or table
name from request JSON.

Required order for a new operation:

1. Begin RC; set transaction-local limits; lock PR21 query revisions; call
   live `personnel.AccessForWrite` and sorted required source locks.
2. Load/lock actual app and current policy, resolve form→table and resource
   existence. Authenticate actor from Session; do **not** require manager.
   Look up actor+operationId and, if confirmed, compare full method/kind/
   route/canonical-body fingerprint and return only the minimum result **before**
   rejecting stale schema/record/draft versions. An active actor can recover
   their own confirmed record/draft operation after data-grant revocation.
3. Claim the shared actor+operationId key. Lock V013's
   `applications.logical_tables` row as the same-table structure gate,
   recheck `schema_ready`, `schema_version`, active field IDs and form binding.
   A pure draft save follows its own owner/form policy and does not bump
   `data_revision`; record writes continue under this gate.
4. Lock the target row for edit, check immutable `created_by` and
   `record_version` CAS, then authoritative local pending-command fence.
   Evaluate complete live `data.create`/`data.edit` tuples for supplied
   fields. For a future task Save, inject a separate trusted task-scoped
   authorizer; do not force ordinary `data.edit` as a second business gate.
5. Normalize formal values through V013's current `appfields.NormalizeValue`,
   apply defaults/required, verify new references against active registry,
   perform controlled typed DML, append minimum audit, consume an exact
   referenced draft if supplied, complete the minimum operation result and
   COMMIT once. Never call Flowable while holding PG locks.

`queryVersion` is checked as an interaction guard using the shared Q36
receipt, never as permission evidence. Read validation may repeat before
business mutation; an executed or ambiguous COMMIT is never retried with a
new operation key. On COMMIT response loss return
`APPLICATION_OPERATION_UNCONFIRMED` plus the original key.

### How the existing V015 Writer must adapt

`services/bff/internal/apprecords/write.go` currently owns dynamic SQL
`INSERT`/`UPDATE` for typed columns. Its `Gate.LockTable` contract already
requires the V013 same-table gate and a schema version recheck; test fixtures
use real PG locks. Its `Authorization` and `Fence` ports fail closed if absent,
and the writer never begins/commits/rolls back. To run with `auth_app`, replace
the direct SQL section with a **controlled typed DML port**; keep the caller
transaction, CAS, fence, authorization and minimum result contract.

Proposed port, subject to owner approval:

```go
type TypedDML interface {
  Insert(ctx context.Context, tx pgx.Tx, trusted Table, recordID, actorID string,
    normalized map[string]appfields.Value) (StoredRecordHeader, error)
  LockHeader(ctx context.Context, tx pgx.Tx, trusted Table, recordID string)
    (StoredRecordHeader, error)
  UpdateCAS(ctx context.Context, tx pgx.Tx, trusted Table, recordID string,
    expectedRecordVersion int64, normalized map[string]appfields.Value)
    (StoredRecordHeader, error)
}
```

`appfields.Value` above denotes the approved canonical/typed V013 value
representation, not a request-supplied SQL expression. Exact Go type can be
chosen by V013 owner. Field ID→physical identifier is derived only from live
registry metadata. **Do not grant `auth_app` generic INSERT/UPDATE/DDL on
`appdata`, generic SQL execution, or ownership of business tables.**

Preferred database capability is an additive SECURITY DEFINER function (or
finite generated per-table functions) restricted to `auth_app`, with fixed
operation enum `insert/update`, actor/app/table/view/record/version IDs and a
canonical field-ID value envelope. It cannot accept SQL fragments, physical
table/column names, or arbitrary table IDs outside the registered app.
**A caller-supplied actor ID or `SET LOCAL app.actor_id` is not proof of the
Redis Session**: `auth_app` can spoof either in a direct SQL call. Before
granting EXECUTE, the shared owner must supply a reviewed transaction-bound
actor capability that the DB can verify but an arbitrary `auth_app` SQL caller
cannot mint (for example, a short-lived HMAC assertion scoped to actor,
authVersion, app/view/table, operation and transaction nonce, with the
verification key inaccessible to `auth_app`). Key custody, replay scope,
rotation and `pgcrypto` availability need explicit review. Under that proof,
the function independently verifies form/table identity, active field
IDs/types, table gate, current grants and fence before using internally
quoted identifiers and typed binds. Its owner/search_path, EXECUTE grants
and all error paths need hostile direct-call tests. Go-side normalization
remains necessary for precise client errors; the DB function cannot trust
Go-side authorization or validation as its sole defense against direct
invocation. If this capability or duplicate normalization is not acceptable,
the lead must choose another reviewed least-privilege boundary before V015
direct DML runs as `auth_app`.

## 3. Candidate hot8 migration and role changes (owner allocation required)

This is a schema delta, **not a migration file**. Scan actual allocation before
naming hot8. Do not alter historical hot7 bytes.

1. Expand `applications.grants` finite CHECK to permit:
   `menu.enter` on existing real menu resources with `row_scope=all` and no
   fields; `data.create` only on `resource_kind='form'`, `row_scope=all`,
   including an explicit empty field mask; `data.read`/`data.edit` only on
   real form views with `row_scope IN ('all','own')`. Preserve one indivisible
   `(app_id,group_id,resource_kind,resource_id,action,row_scope)` tuple.
   No wildcard, subordinate, deny, inherited directory or cross-app tuple.
2. Remove `ck_b5_no_grant_fields` only after adding a protected field relation:
   `grant_fields.table_id uuid NOT NULL` (existing rows are empty), FK
   `(app_id,table_id,field_id)` to `applications.fields`, and a trigger or
   equivalent locked validation that the grant is a data action on an actual
   form whose `table_id` equals this row and the field is not removed.
   Menu grants must have zero field rows. An active grant must not silently
   survive a field tombstone as a phantom permission: coordinate definition
   Save to block removal or atomically retire affected field grants and bump
   `policy_revision`, pending lead choice. All grant changes use the existing
   app/policy lock and revision, including empty-mask create.
3. Add `applications.record_drafts` keyed by UUID, with owner/app/table/view,
   nullable target/base pair, schema version, draft version, JSONB *incomplete*
   payload, created/updated times and owner cursor index
   `(owner_user_id,app_id,view_id,updated_at DESC,id DESC)`. Composite
   app/table/view FKs must ensure same-app binding; dynamic target row binding
   is checked under the table gate. No automatic expiry, purge or quota is
   invented. `auth_app` gets only the reviewed draft table privileges.
4. Extend `applications.operations.operation_kind` with
   `record.create`, `record.edit`, `draft.create`, `draft.update`,
   `draft.discard`. For the external 204 discard, permit `http_status=204`
   while keeping a non-null internal minimum `result_json` object; the HTTP
   response itself has no body. Record create/edit result is precisely
   `{operationId,id,recordVersion,schemaVersion,createdAt,updatedAt}`.
   Draft create/update/discard ledger result contains no payload values.
   Extend `readableOperation` by kind: live authenticated **same actor** may
   read their own minimum record/draft confirmation after field grants are
   revoked; retain manager rules for existing manager kinds. Same actor/key
   with a changed fingerprint remains conflict across all kinds.
5. Add restricted `applications.record_write_audit` for the frozen **minimum**
   record audit: actor/app/table/view/record, operation ID, before/after
   recordVersion, sorted changed field IDs, origin and occurredAt. It has no
   old/new business values or raw request JSON and is inserted in the same
   transaction as typed DML and operation completion. This is distinct from
   the separately proposed old/new business history and must not be called
   a complete financial change log.
6. Add a real `applications.record_command_fences` relation or flow-owned
   equivalent keyed by actual app/table/record/command. Store at least
   `state`, `expected_record_version` and command identity. Index unresolved
   rows by `(app_id,table_id,record_id)`; never infer absence from a missing
   relation or failed query. Both pending-command creation/transitions and
   ordinary record writes take the **same V013 table gate** before inspecting
   or changing fence state. A present pending row returns
   `APPLICATION_RECORD_FENCED`; an available empty table proves no pending;
   missing/unavailable guard returns 503. Flow owner retains command-state
   authority; no automatic completion or retention policy is implied.
7. Add narrow indexes after EXPLAIN: typed table `(created_by,id)` and
   `(created_at DESC,id DESC)` where not already present; grant tuple and
   grant field lookup; draft owner cursor; fence unresolved lookup. Dynamic
   numeric/date/time/ref field indexes require workload evidence and DDL
   coordination, not one index per user field by default. The additive
   migration must cover both already-created hot7 typed tables and future
   table creation through a reviewed V013 DDL function update. No A-query journal
   or cold archive is included. Restrict audit/result/raw draft roles and
   preserve backup access according to existing reviewed patterns.

### Reference source delta still needed

V013's `appstructure/references.go` exposes an injected validation port for
definition defaults; `member_candidates.go` is an owner-only selector. Neither
is a durable application reference registry or source-version hook.
Personnel's actual member model is `auth.users` plus
`personnel.department_members`, where one member has **multiple** department
IDs. The source owner must register immutable member/department IDs and
tombstones with last display, maintain normalized member↔department edges,
and advance a source revision transactionally from every relevant personnel
create/rename/status/membership/delete path. The app record writer validates
**new** reference values against active source; an old tombstoned reference
keeps its UUID and last display. No deleted reference is silently set NULL
and no member picks a first department.

For queries without reference predicates, count/page the authorized typed
records first and resolve distinct page reference IDs in a batch in the same
RR snapshot. A reference predicate must join/EXISTS against authoritative
registry/edge facts **before COUNT/LIMIT** in that snapshot. The projection
revision vector includes the source revision for displayed references. An
unavailable registry/hook returns service unavailable; an absent revision
cannot be interpreted as unchanged. This registry/edge/hook migration and
personnel writer edits need explicit shared/source ownership; B4a local
package alone is not a production HTTP connection.

## 4. Q36 shared lifecycle extraction and exact path ownership

The currently executable Q36 algorithm is not just `FingerprintRows`: it has
Session-bound Redis context, revision CAS, RR snapshot, criteria comparison,
full projection validation, page-only path, context eviction, and a write
receipt. V015 currently provides only dynamic exact filter compiler and
bounded streaming hash in `services/bff/internal/appquery`.

**Shared owner edits needed before calling A implemented:**

| Exact path | Required change / ownership |
| --- | --- |
| `services/bff/internal/personnel/query_context.go` | Extract finite Redis Create/Load/Advance, per-Session LRU/TTL and CAS into a neutral backend (suggest `services/bff/internal/querycontext/`). Preserve existing personnel key format and limits through a wrapper; parameterize context kind, criteria validator and revision vector. V015 does not edit this personnel file. |
| `services/bff/internal/personnel/query_engine.go` | Route personnel through the shared RR/revision/fingerprint/page lifecycle without changing existing HTTP behavior; expose a strategy interface for appquery's compiler, live policy and typed projection. V015 does not edit this file. |
| `services/bff/internal/personnel/query_write_guard.go` | Expose a shared query receipt for list-origin writes: verify old context in RR, then recheck revisions in the new RC write tx before mutation. Limit retries to *pre-execution* validation. V015 does not edit this file. |
| `services/bff/internal/personnel/query_context_test.go`, `query_projection_test.go`, `query_write_guard_test.go` and HTTP tests | Preserve Q36 context keys, changed/expired distinction, CAS and write behavior with real Redis/PG. Shared owner owns regression edits. |
| `services/bff/internal/applications/transactions.go`, `store.go`, `http.go`; `services/bff/cmd/bff/config.go` | Supply non-manager lifecycle, operation routing and real app HTTP composition. V013 owner controls these files. |
| `db/migrations/00008_*.sql`, `infra/runtime/roles.sql`, `contracts/errors/codes.json`, `contracts/openapi/openapi.json` | One shared owner allocates schema/roles/error/API edits. V015 supplies delta only. |

The neutral context metadata for records is
`{kind:'apprecords',appId,tableId,viewId,criteriaCanonical,total,fingerprint,
observed:{dataRevision,policyRevision,schemaVersion,viewVersion,
dependencyRevision,registryRevision},protocolVersion}`. It stores no result
IDs or field values. The token is random, Session-bound, with Q36 bounded LRU
and idle expiry. Context from another app/table/view or changed Session is
expired/invalid; no context is authorization. Registry revision is required
whenever reference display is in the projection; missing hook is unavailable,
not unchanged.

For `POST /records/search`, load context metadata before PG RR, then in one
RR snapshot revalidate Session, actual form/table, live policy, registry and
source revisions; normalize criteria and check full field scope coverage.
If an old token is supplied, validate its **saved old criteria first**, as
Q36 does, even when the incoming criteria differ. If its observation
revisions differ, stream that old criteria's full projection and return
`QUERY_CHANGED` only when digest/count differ; otherwise CAS-advance. If
the old criteria can no longer compile because a field was tombstoned,
return `QUERY_CHANGED` to an otherwise authorized actor. If a filter/sort
field lost full read-scope coverage, reject 403 immediately. Once the old
context is valid, changed criteria stream a new full authorized projection,
COUNT and page and create a new context; unchanged criteria with unchanged
revisions use saved total and page-only query. Commit RR
before Redis Create/Advance. A CAS race re-reads/revalidates bounded times or
returns context expired/busy; it cannot publish an unverified page. Explicit
refresh sends **no old queryVersion and page=1**. A valid page beyond total
returns empty `items`; offset remains `O(offset+pageSize)` even with index.

The matching authorized projection streams ID, order, all currently readable
field values for the form's shared table and observable reference display;
layout and hidden columns do not narrow it. Permission scope is checked
before filter/COUNT/page. A changed policy that removes access rejects
immediately; it does not compare or serialize an old field. `appquery.Compile`
provides typed predicates and referenced field IDs;
`appaccess.CoversRead` gates filter/sort before restricted predicates;
`appquery.FingerprintRows` hashes a server-side ordered JSONB row stream with
bounded Go memory. These primitives become strategy inputs after extraction,
not an independent personnel adapter or duplicate Redis engine.

One projection detail needs lead confirmation before wire integration:
`Record` exposes `recordVersion` and `updatedAt`, while ADR §8.1 explicitly
names IDs/order/count/authorized field values/reference display for P. I
recommend including observable system `recordVersion`/`updatedAt` in P so
an off-page matching row cannot change a returned Record without changing
queryVersion. Under that choice, a write only to a hidden business field may
still be relevant because its version/time change is visible. If the lead
intends such writes to leave the query context unchanged, it should explicitly
freeze the narrower P and the Table's treatment of system metadata. The
current hash primitive is input-agnostic and does not silently choose this.

## 5. Separate quick-search proposal for lead decision

Proposed additive `RecordSearch.quickSearch?: {text:string;fieldIds:UUID[]}`.
Only explicit active text/multiline field IDs are searched, 1–20 unique IDs,
same table/form; text is 1–160 Unicode scalar values after trimming outer
Unicode whitespace, with NUL/unpaired UTF-16 rejected. Proposed matching is
**case-sensitive literal substring of stored text** using parameterized
`strpos(field, $term)>0`; `%`, `_`, `\\` and regex characters are literals.
No implicit search over secret fields, reference labels, audit or all schema
fields. Each selected field's read scope must cover every visible row or the
whole request is 403 before evaluation. Results satisfy quick-search OR
across selected fields AND the existing structured filter. Canonical sorted
field IDs/text become part of query criteria/fingerprint; term or field set
change creates a new context and page 1. No auto-complete or fuzzy ranking.

This proposal deliberately selects deterministic literal semantics; full
scan may cost `O(N × selected text fields × term length)` without a reviewed
index. A trigram/index or Unicode case-insensitive variant requires separate
cost/locale/collation-version review. Lead must approve whether literal
case-sensitive substring and explicit field list meet the Figma quick-search
interaction, or choose a different precise rule. **No quick-search code or
OpenAPI change is authorized by this proposal alone.**

## 6. Separate old/new business history proposal for lead decision

Keep minimum actor/app/table/record/version/operation/changed-field-ID audit
with the record transaction. To satisfy the financial old/new example,
propose separate restricted `applications.record_change_events` and
`record_change_values` (precise candidate columns and wire route in
[contract-and-algorithms.md](contract-and-algorithms.md#separate-record-change-history-privacyretention-proposal-for-freeze)).
Changed values are canonical per-field old/new only, keyed by event and
field ID, never in global personnel authentication audit, operation result,
log or ordinary query context. One version-changing write, including future
task Save, appends an event in the same transaction; no-op has no value delta.

Proposed GET history requires current actual form existence and current row
`data.read` (404 for unreadable row); each changed field is masked by the
current complete read tuple. Hidden-only events are removed **before**
LIMIT/cursor so actor/time/version do not reveal them. Mixed events expose
only permitted field deltas. Cursor binds actor/Session/form/record and
policy/schema revision. Historical source display is resolved from current
registry/tombstone and must not imply immutable old labels.

Lead decisions required before implementation: `data.read` alone versus a
new history action, exposure of actor/time on mixed events, retention and
backup privacy policy, sensitive-field exclusions, and task reference
visibility. No purge, archive deadline or old-value storage is inferred.

## 7. Acceptance still required

V015 temporary-table tests and the core scale runs are not a live policy
or real API proof. After the shared owner lands the above ports: migrate a
fresh isolated PG18.6 database; test as restricted `auth_app` and hostile
direct calls to the controlled DML capability; real grant/revoke races;
record/draft/operation/audit atomicity and unknown COMMIT; authoritative
empty/pending fence; actual member multi-department source and tombstones;
same-RR reference predicates; 10k/100k/1m × 1/20/200 context costs; HTTPS
Session/CSRF/OpenAPI; Chromium/Firefox/WebKit against the actual PG-backed
Table flow. Do not mask a failing million-row case by enlarging timeout.
