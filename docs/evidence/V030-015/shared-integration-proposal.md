# V030-015 shared integration delta for V030-013 owner

Status: **reviewable proposal, not migrated or available through HTTP**.
Authoritative behavior is the [frozen V030-015 ADR §8](https://app.notion.com/p/3ee2f5a9e6488163a144fba5f55e3c14).
This delta was rechecked against V030-013 remote
`bf4956394581979f3371a8840a95f3c59166b801` on 2026-10-03. Its latest
department-candidate/permission-matrix additions do not supply the V015
non-manager write/grant/context ports below. V015 worktree remains based on
the fixed PR26 head and
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
  INSERT/UPDATE on business tables. V015's `apprecords.Writer` now requires a
  `TypedDML` port. Its direct SQL adapter exists only in the PG owner test
  fixture; no controlled runtime adapter exists yet for the reviewed role.
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

### V015 Writer controlled DML port, ready for owner adapter

`services/bff/internal/apprecords/write.go` now requires a `TypedDML` port;
nil fails closed. It contains no direct `INSERT`/`UPDATE` fallback. Its
`Gate.LockTable` requires the V013 same-table gate and schema recheck;
`Authorization` and `Fence` still run before mutation. The writer owns no
Begin/Commit/Rollback. Its PG owner-role **test fixture only** implements the
port with direct SQL, preserving CAS and concurrency coverage until the V013
owner supplies a controlled runtime adapter. The actual Go port is:

```go
type TypedDML interface {
  Insert(context.Context, pgx.Tx, Table, Create, []string) (StoredHeader, error)
  LockHeader(context.Context, pgx.Tx, Table, string) (StoredHeader, error)
  UpdateCAS(context.Context, pgx.Tx, Table, Edit, []string) (StoredHeader, error)
}
type StoredHeader struct {
  ID, CreatedBy string
  RecordVersion int64
  CreatedAt, UpdatedAt time.Time
}
```

`Create.Values` and `Edit.Changes` are provided only after the BFF uses V013's
current field normalizer; the selected IDs are validated against trusted
active metadata before this port. The owner adapter must revalidate canonical
types/field IDs in its controlled database capability, derive physical
identifiers only from live registry metadata, and ignore request-supplied
namespace or SQL text. **Do not grant `auth_app` generic INSERT/UPDATE/DDL on
`appdata`, generic SQL execution, or ownership of business tables.**

The lead fixed the trust boundary on 2026-10-03: the Redis Session/BFF is
responsible for end-user identity and authorization, and `auth_app` is a
trusted service role. A database function using this shared role **does not
prove which end user owns a Session**. No new HMAC actor assertion, key
custody or rotation system is proposed. BFF must derive actor from the live
Session, enforce CSRF, load complete current grants, fence and schema/record
versions, and do those checks inside the same caller-owned transaction before
invoking typed DML. Operation replay/unknown-result recovery remains at BFF.

The preferred database capability is an additive SECURITY DEFINER function
(or finite generated per-table functions) restricted to `auth_app`, with
finite `lock_header/insert/update` operations and actor/app/table/view/record/version IDs
plus a canonical field-ID value envelope from BFF. It must not accept SQL
fragments or physical table/column names, grant arbitrary DDL, update system
columns supplied by the caller, or reach tables outside the registered real
app/table/view. It independently validates registered table and active field
IDs/types, canonical values, immutable createdBy, recordVersion CAS and the
same-table structure gate before internally quoting identifiers and binding
typed values. `updatedAt` and `recordVersion` advance together for every
actual business value mutation; schema conversion bumps schemaVersion.
`LockHeader` must itself be a controlled capability: PostgreSQL `SELECT ...
FOR UPDATE` needs write privilege, which the reviewed `auth_app` role does
not have on dynamic business tables. Do not solve that by broad table UPDATE
grants.
The function's owner/search_path, EXECUTE grants and direct-call boundary
tests must show no generic SQL or DDL path, no system-column bypass and no
cross-table write. A direct SQL call under the trusted `auth_app` role can
impersonate an actor argument; that is an explicit consequence of this trust
boundary, not a claimed database Session proof. The BFF transaction is the
enforcement point for live end-user grant, CSRF and pending-command fence.

## 3. Candidate hot8 migration and role changes (owner allocation required)

This is a schema delta, **not a migration file**. Scan actual allocation before
naming hot8. Do not alter historical hot7 bytes.

1. Expand `applications.grants` finite CHECK to permit:
   `menu.enter` on existing real menu resources with `row_scope=all` and no
   fields; `data.create` only on `resource_kind='form'`, `row_scope=all`,
   including an explicit empty field mask; `data.read`/`data.edit`/
   `data.history` only on
   real form views with `row_scope IN ('all','own')`. Preserve one indivisible
   `(app_id,group_id,resource_kind,resource_id,action,row_scope)` tuple.
   No wildcard, subordinate, deny, inherited directory or cross-app tuple.
2. Remove `ck_b5_no_grant_fields` only after adding a protected field relation:
   `grant_fields.table_id uuid NOT NULL` (existing rows are empty), FK
   `(app_id,table_id,field_id)` to `applications.fields`, and a trigger or
   equivalent locked validation that the grant is a data action on an actual
   form whose `table_id` equals this row and the field is not removed.
   Menu grants must have zero field rows. An active grant must not silently
   survive a field tombstone as a phantom permission: definition Save **blocks
   and identifies dependent grant IDs** until an administrator explicitly
   revokes them, per §8. `data.history` starts with no grants. All grant
   changes use the existing
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
7. Add narrow indexes after EXPLAIN: typed table
   `(created_by,created_at DESC,id DESC)` and
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
`appquery.CompileAccess` now produces the row predicate and permission-
conditioned field JSONB projection: an own-only field is emitted only in
the actor-created row, and unreadable keys never reach Go; a real PG test
checks both all+own masks and own COUNT. `appquery.CompileSearch` combines
this with typed filter/sort only after whole-visible-scope coverage succeeds;
its RED/GREEN test rejects an own-only secret filter/sort across all rows.
`appquery.FingerprintRows` hashes a
server-side ordered JSONB row stream with bounded Go memory.
`appquery.PageWindow` guards arbitrary OFFSET arithmetic. These primitives
become strategy inputs after extraction,
not an independent personnel adapter or duplicate Redis engine.

The lead confirmed that observable `Record` metadata, including
`recordVersion` and `updatedAt`, belongs in P alongside ID/order/count,
readable business fields and readable reference `(id,label,deleted)` display.
Any version-changing edit to a matching row is therefore relevant even if
only an unreadable business field changed. A nonmatching row or unrelated
resource still must not force a **user-visible** refresh. Selected A may
still need an expensive complete rehash after a same-table revision bump;
that read cost is separately measured. `FingerprintRows` is input-agnostic;
the final SQL strategy must include these system fields explicitly.

## 5. Lead-selected quick-search contract, pending source/API writeback

Add `RecordSearch.quickSearch?: {term:string;fieldIds:UUID[]}`. Require 1–20
distinct current `text`/`multiline` IDs on this form/table. Trim outer
Unicode whitespace; then require 1–160 Unicode scalar values, valid Unicode
and no NUL. Matching is a **literal substring with ASCII A–Z folded to a–z
on both sides**. All other Unicode code points keep literal semantics; do
not use database locale casefold or invent fuzzy ranking. A parameterized PG
shape is `strpos(translate(r.f_<trusted_id>, 'ABCDEFGHIJKLMNOPQRSTUVWXYZ',
'abcdefghijklmnopqrstuvwxyz'), translate($term, 'ABCDEFGHIJKLMNOPQRSTUVWXYZ',
'abcdefghijklmnopqrstuvwxyz')) > 0`. `%`, `_` and backslash are ordinary
characters, never LIKE patterns. NULL does not match.

OR the selected fields, then AND the structured filter. Before SQL execution,
each selected field's read scope must cover the entire visible row scope; any
failure rejects the whole request with 403. Never search hidden fields,
reference labels, audit, layout text, system fields or all schema fields by
default. Canonical criteria sort field IDs and record the ASCII-folded term;
changing either starts a new context and page 1. No implicit empty-term
search is defined: reject an empty term, or omit `quickSearch` entirely.
`strpos` can scan `O(N × selected text fields × term length)`; ordinary BTree
does not make arbitrary contains search indexed. Measure EXPLAIN and write
cost before choosing any specialized index. This is a lead-selected contract
direction, not implemented SQL/OpenAPI yet.

An [isolated PG18.6 probe](quick-search-cost.txt) with 1,000,002 text rows
and a BTree on the text column measured about 5.0–5.3 seconds for each
single-field count, whether `PREFIX` matched one million rows or `999999`
matched one. EXPLAIN used Seq Scan in both cases. `%_\\` matched literally,
and lowercase `éclair` did not match uppercase `Éclair`, while `ÉCLAIR` did.
This one-run fixture is evidence **against assuming the existing BTree helps**;
it is not a p95 SLA or approval for a trigram/index migration. Twenty OR
fields and concurrent load remain unmeasured.

## 6. Lead-selected history authorization direction; exposure still gated

Keep minimum actor/app/table/record/version/operation/changed-field-ID audit
with the record transaction. To satisfy the financial old/new example,
propose separate restricted `applications.record_change_events` and
`record_change_values` (precise candidate columns and wire route in
[contract-and-algorithms.md](contract-and-algorithms.md#separate-record-change-history-privacyretention-proposal-for-freeze)).
Changed values are canonical per-field old/new only, keyed by event and
field ID, never in global personnel authentication audit, operation result,
log or ordinary query context. One version-changing write, including future
task Save, appends an event in the same transaction; no-op has no value delta.

Add independent `data.history` complete grant tuples on the real form, default
no grant; owner/Bootstrap retains full capability only for an existing real
resource. A history read requires both current row `data.read` and a
field-by-field intersection of current `data.read` and `data.history`
row-scope/field masks. A caller without row read gets 404; a caller with no
history action gets 403. Filter events with zero permitted deltas **before**
LIMIT/cursor, not in the BFF after pagination. Mixed events reveal only
permitted deltas plus that event's actor/time; hidden field IDs, counts and
hidden-only event headers are absent. Cursor binds actor/Session/form/record,
policy and schema revision. A task reference is emitted only if the current
actor can access that actual task; otherwise omit task details entirely.

`old_value` and `new_value` are canonical typed wire values (or JSON null),
with `field_kind_at_write` retained on the restricted event delta. Option and
reference values store stable IDs, not labels. If a label is needed at read,
resolve current active/tombstone label; mark deleted as appropriate and never
claim it is the historical label at event time. If an option has no retained
label, return its stable ID with an unavailable-label marker, never guess a
new option. A removed field's historical value is not made readable to an
ordinary actor by an old grant: field deletion first requires explicit grant
revocation, so current masks exclude it. Owner/Bootstrap rendering of a
removed field would use the retained field tombstone and event-time kind;
precise label and field-removal history presentation still need final review.

The events/values tables are separate from global authentication audit,
operation receipts and ordinary logs. Backups use the existing encrypted
backup controls and a minimum role; the global auth-audit reader gains no
business-value access. With no automatic purge or new retention days, data,
index, WAL and encrypted-backup growth is unbounded over time:
`O(sum(changed canonical value bytes + per-delta/event overhead))`. No
retention cap or cold archive is silently inferred. A V013 schema conversion
that changes stored values but only advances schemaVersion is **not yet a
per-record old/new event**. Before claiming complete business history or
exposing a history API, the lead must decide whether to add atomic per-row
conversion deltas/version bumps (potentially O(N) writes and WAL per schema
Save) or explicitly limit history to record/task Save mutations. Also confirm
removed-field presentation and option tombstone retention. The history route
remains proposal-only until these are frozen and source/API writeback is done.

## 7. Ordinary-user runtime form and reference-display contract

V013's current definition read is owner-only. It cannot be the Table or
V014 FieldRenderer data source for an ordinary actor. Proposed additional
`GET /api/v1/applications/{appId}/forms/{viewId}/runtime` returns the
following **sanitized runtime projection** under actual Session and form
`menu.enter`, plus at least one current `data.read/create/edit` action. A
create-only actor must be able to load a form; a menu-only actor gets 403.
Owner/Bootstrap still needs the real resource to exist. This endpoint is a
separate read of runtime metadata, not a loosening of the manager definition
route.

```ts
type Scope = "none" | "own" | "all";
type RuntimeField = {
  id: UUID; name: string; kind: FieldKind; required: boolean;
  presentation: {helpText: string | null; displayTimeZone: string | null};
  // Curated input constraints only; no raw fields_json or dependency config.
  input: {decimal?: {precision:number;scale:number;roundingPlaces:number;
      roundingMode:string}; timePrecision?: "second" | "millisecond";
    options?: {id:UUID;label:string}[];
    referenceKind?: "member" | "department"};
  default?: FieldValue; // only when this field is create-authorized
  access: {read:Scope; create:boolean; edit:Scope};
  query: {operators:("eq"|"neq"|"gt"|"gte"|"lt"|"lte")[];
    sortable:boolean; quickSearchable:boolean};
};
type RuntimeView = {
  appId:UUID; tableId:UUID; viewId:UUID;
  schemaVersion:Version; viewVersion:Version; policyRevision:Version;
  fields:RuntimeField[];
  layout: LayoutNode[]; // V013 field/group/description/system_field shape
  capabilities:{create:boolean; read:Scope; edit:Scope;
    search:boolean; draftCreate:boolean; draftEdit:boolean};
};
type ReferenceDisplay = {id:UUID; label:string; deleted:boolean};
type BusinessRecord = {
  // Existing frozen id/app/table/view/system/version/values members remain.
  referenceDisplays: {[fieldId:UUID]: {[sourceId:UUID]:ReferenceDisplay}};
};
```

`fields` includes only the union of currently create/read/edit-authorized
active fields. Its action scopes come from complete actual-form tuples, not
mixed scope and field grants. Drop unauthorized field nodes from `layout`,
then prune empty groups; only curated display-safe description/system nodes
remain. Query operators, sortability and quick-searchability require
read-scope coverage over every visible row, not merely a field's partial
read grant. No field definition, default, options, reference candidates,
owner-only workflow/DDL/dependency configuration or hidden field name for a
field with no action permission crosses the response. `schemaReady=false`
returns the frozen schema-not-ready conflict, not a fabricated empty form.
Use `RuntimeField` and sanitized `layout` as V014 FieldRenderer props with
the existing `values[fieldId]`, field errors and change callback; use
`access` to choose view/create/edit state and query capability, rather than
creating a parallel field renderer or trusting client permissions on submit.

GET Record and each RecordPage item add `referenceDisplays` only for IDs in
their **readable** `values` fields. Scalar member/department refs have one
entry; any future multi-ref field uses the same fieldId→sourceId map. Fetch
distinct page IDs in a batch after authorized COUNT/page, in the same RR
snapshot; never return a whole personnel DTO or all candidate members.
Deleted source IDs retain their stable UUID, last canonical label and
`deleted=true`. Missing registry/tombstone data is unavailable/integrity
failure, not an empty label. A reference predicate still joins registry
before COUNT/LIMIT. The complete projection P includes exactly the same
observable `(fieldId,sourceId,label,deleted)` values for every matching row;
changing a visible label or deleted bit is relevant, while changing a source
that no matching readable row references is not. This mapping also defines
the compact-A source-digest experiment below.

## 8. Field-removal dependency and lock order for V013 owner

The lead selected **block then explicit revoke**: when a definition Save
would tombstone field X, query current `grant_fields` for any data grant that
references X and reject Save with only the related grant IDs/resources as
authorization dependencies. An administrator explicitly revokes those
grants through the normal policy transaction, then retries definition Save.
Schema Save must never silently delete or narrow data grants. The dependency
check runs under the same app/policy and logical-table locks as the mutation,
so a concurrent grant replacement cannot add a grant between check and
field tombstone.

Canonical order for grant replacement, definition Save and record writes is
PR21 personnel revision lock → required source locks (sorted) → app/policy
row gate → logical-table row gate → grant/field rows → record/fence rows.
V013's existing manager transaction begins with personnel revision lock,
then app row gate; V015 ordinary writes must respect the same order. Grant
replacement touching a field also takes that field's logical-table gate
after the app gate. Field deletion checks grant dependencies under both
gates, returns conflict, and makes **no** schema/policy/data revision change
on rejection. Explicit revoke advances policyRevision; subsequent Save
advances schemaVersion and preserves grant audit. Cross-regression must race
Save-vs-grant-add, Save-vs-revoke, and record edit-vs-Save to prove no
deadlock, phantom field permission or half-committed schema/grant state.
An already removed field cannot be granted again; active grant-field
validation locks the field registry and checks tombstone state.

## 9. Acceptance still required

V015 temporary-table tests and the core scale runs are not a live policy
or real API proof. After the shared owner lands the above ports: migrate a
fresh isolated PG18.6 database; test as restricted `auth_app` and hostile
direct calls to the controlled DML capability; real grant/revoke races;
record/draft/operation/audit atomicity and unknown COMMIT; authoritative
empty/pending fence; actual member multi-department source and tombstones;
same-RR reference predicates and page display; RuntimeView ordinary
menu/create/read/edit combinations; field removal vs grant-add/revoke races;
history row/field intersection and before-LIMIT masking after its remaining
exposure choices are frozen; 10k/100k/1m × 1/20/200 context costs; HTTPS
Session/CSRF/OpenAPI; Chromium/Firefox/WebKit against the actual PG-backed
Table flow. Do not mask a failing million-row case by enlarging timeout.
