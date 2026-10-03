# V030-015 record/query/draft incremental contract and algorithm review

Status: **Notion PRD frozen and ADR §8 accepted on 2026-10-03; selected core
contract released for implementation. This file remains an integration proposal,
not evidence of API availability.**
Source and branch provenance: [task record](../../tasks/V030-015.md).
The V030-015 PRD and ADR §8 are the accepted source for record/query/draft
semantics. V030-013's separate authoritative ADR freezes structure/field
semantics; this document only extends those boundaries. The lead selected
Candidate A. Quick search and business old/new history remain proposal only.

## 1. Actual code facts and integration gaps

1. PR26 head 9b89e8e has application ownership, group membership, a durable
   actor+operationId ledger, live AccessForWrite, and a menu-only resource
   registry. In applications/store.go, normalized(grants.replace) accepts only
   resourceKind=application/action=menu.enter/rowScope=all/empty fields. Hot
   migration 00006 enforces the same: menu_resources only has the app root,
   grants action CHECK menu.enter, row_scope CHECK all, and grant_fields
   CHECK(false). Its snapshot does not load data field masks. The B4b
   apppolicy evaluator supports only menu.enter, data.read, data.edit on
   caller-supplied coherent facts, with no SQL/auth/cache. It does not yet
   support a data.create action. Therefore present B5 HTTP permissions **cannot**
   authorize ordinary record writes or reads by pretending a group has them.
2. PR26 personnel/query_engine.go is an authorized RR snapshot plus Redis
   query-context implementation. It persists normalized criteria, total,
   complete-result SHA-256 fingerprint and people/configuration/activity
   revisions. If a revision changes, it recomputes the complete projection and
   returns QUERY_CHANGED only for a real fingerprint/total difference. When
   unchanged it uses a page-only path. Filter compiler in query_filter.go is
   restricted to members/events and fixed personnel columns; it is not a
   generic dynamic-field SQL compiler. Existing web Table uses server
   pagination/sort, queryVersion and explicit refresh; hiddenColumnIds are
   presentation only. This is the unified *interaction and context* abstraction
   to reuse, with a shared server query lifecycle, not a second frontend table.
3. Existing personnel drafts are owner-user-scoped, explicit PG saves with
   draft CAS and exact-version consumption inside business writes. They do not
   change query revisions. The application draft needs schema/view/resource
   binding and the shared operation ledger, so copying their table verbatim is
   insufficient. No arbitrary expiry/quota is inferred from personnel limits.
4. Real personnel member lookup aggregates all department IDs from
   personnel.department_members; members.go/people.go expose DepartmentIDs[].
   It is incorrect to choose one department. Current PR26 tree has no apprefs
   module, no personnel source-version hooks and no production registry
   migration. The B4a package exists only as an independently delivered local
   module per the v0.3.0 PRD/ADR; its source was not present in this fixed
   checkout. Its measured/claimed behavior is not evidence of live integration.
   The V030-013 269c774 branch has an appschema prototype and proposal, but
   no released full HTTP/metadata integration at this inspected SHA. Its ADR
   requires trusted typed columns, field normalization, same-table gate,
   controlled DDL, caller-owned ApplyInTx and shared operation lifecycle.
5. Hot 00001–00006 and cold 00001–00003 exist at PR26. V030-013 reserves
   hot 00007 and cold 00004 in its authoritative ADR; neither file existed at
   the inspected V030-013 SHA. V030-015 proposes **no migration file** now.

Relevant source: [V030-015 PRD](https://app.notion.com/p/3ee2f5a9e6488116be71c0a0c332a27a),
[V030-015 ADR](https://app.notion.com/p/3ee2f5a9e6488163a144fba5f55e3c14),
[V030-013 ADR](https://app.notion.com/p/3ee2f5a9e64881489017e2f38a4bb29f),
[v0.3.0 PRD](https://app.notion.com/p/3ed2f5a9e648811abf4ee555fb9695aa),
[accepted ADR-002](https://app.notion.com/p/3e52f5a9e648813da842e2ef15a09b3a).

## 2. Frozen core HTTP delta and separately proposed extensions

The record/query/draft routes below are frozen beneath /api/v1/applications/{appId}. Their
{viewId} is a V030-013 form view; its actual tableId and app owner are loaded
from storage, never trusted from body. A record is the one shared table row,
not a row copy per view. Standard JSON response is the ADR-002
code/message/data/meta envelope; requestId appears in meta, no-store is set,
write routes require actual Session, origin and CSRF protection. UUIDs are
lowercase canonical. Body rejects unknown/duplicate keys, lossy JSON numbers
and unsupported field kinds. The wire values map by stable fieldId only; it
does not define business JSONB storage.

| Method/path suffix | Request data | Success data/status |
| --- | --- | --- |
| POST /forms/{viewId}/records | RecordCreate | MutationResult/201 + Location |
| GET /forms/{viewId}/records/{recordId} | none | Record/200 |
| PATCH /forms/{viewId}/records/{recordId} | RecordEdit, application/json | MutationResult/200 |
| POST /forms/{viewId}/records/search | RecordSearch | RecordPage/200 |
| POST /forms/{viewId}/drafts | DraftCreate | DraftMutationResult/201 + Location |
| GET /forms/{viewId}/drafts | pageSize,pageToken query | DraftSummary[]/200 + meta.pagination |
| GET /forms/{viewId}/drafts/{draftId} | none | Draft/200 |
| PATCH /forms/{viewId}/drafts/{draftId} | DraftUpdate, application/json | DraftMutationResult/200 |
| DELETE /forms/{viewId}/drafts/{draftId} | operationId,expectedDraftVersion query | 204 |
| GET /api/v1/application-operations/{operationId} | existing route | additive record/draft result kinds |

Proposed wire types, all shown keys required unless suffixed ?:

    type UUID = string; type Version = number; // safe JSON integer 0..2^53-1
    type FieldValue = string | boolean | string[] | null;
    // Exact kind drives validation: decimal is a decimal string; date YYYY-MM-DD;
    // datetime explicit-offset RFC3339 normalized UTC; option/ref stable UUID.
    // Only multi_select uses string[]; null is explicit clear.
    type Values = {[fieldId:UUID]:FieldValue};
    type BusinessRecord = {id:UUID; appId:UUID; tableId:UUID; viewId:UUID;
      createdBy:UUID; createdAt:string; updatedAt:string;
      recordVersion:Version; schemaVersion:Version; values:Values};
    type MutationResult = {operationId:UUID; id:UUID;
      recordVersion:Version; schemaVersion:Version;
      createdAt:string; updatedAt:string};
    type DraftRef = {id:UUID; draftVersion:Version};
    type RecordCreate = {operationId:UUID; expectedSchemaVersion:Version;
      values:Values; draftRef?:DraftRef; queryVersion?:string};
    type RecordEdit = {operationId:UUID; expectedSchemaVersion:Version;
      expectedRecordVersion:Version; changes:Values;
      draftRef?:DraftRef; queryVersion?:string};
    type FilterGroup = {operator:"and"|"or";
      children:(FilterGroup|FilterCondition)[]};
    type FilterCondition = {fieldId:UUID|"createdAt"|"updatedAt";
      operator:"eq"|"neq"|"gt"|"gte"|"lt"|"lte";
      value:FieldValue};
    type Sort = {fieldId:UUID|"createdAt"|"updatedAt";
      direction:"asc"|"desc"} | null;
    type RecordSearch = {page:Version; pageSize:Version;
      filter:FilterGroup|null; sort:Sort; queryVersion?:string};
    type RecordPage = {items:BusinessRecord[]; total:Version; page:Version;
      pageSize:Version; sort:Sort; queryVersion:string;
      schemaVersion:Version; viewVersion:Version};
    type DraftValue = string | boolean | string[] | null;
    type DraftValues = {[fieldId:UUID]:DraftValue};
    type DraftSummary = {id:UUID; viewId:UUID; tableId:UUID;
      targetRecordId:UUID|null; schemaVersion:Version;
      baseRecordVersion:Version|null; draftVersion:Version;
      createdAt:string; updatedAt:string; hasConflicts:boolean};
    type DraftConflict = {fieldId:UUID|null;
      reason:"FIELD_REMOVED"|"FIELD_PERMISSION_REVOKED"|
        "SCHEMA_CHANGED"|"BASE_RECORD_CHANGED"};
    type Draft = DraftSummary & {values:DraftValues;
      conflicts:DraftConflict[]};
    type DraftMutationResult = {operationId:UUID; draftId:UUID;
      draftVersion:Version; updatedAt:string};
    type DraftCreate = {operationId:UUID; targetRecordId:UUID|null;
      schemaVersion:Version; baseRecordVersion:Version|null;
      values:DraftValues};
    type DraftUpdate = {operationId:UUID; expectedDraftVersion:Version;
      changes:DraftValues; removeFieldIds:UUID[]};
    type DraftDeleteQuery = {operationId:UUID;
      expectedDraftVersion:Version};

Create is sparse: omitted fields use V030-013 normalized constant defaults;
required fields must be non-null after defaults. Edit is an explicit changed-field
map: omitted fields retain values, null clears an optional field, empty changes
is a no-op with an idempotent operation result, minimal operation audit and no
recordVersion bump. PATCH is a closed application/json command: changes lists
the only fields to replace, null explicitly clears an optional field, and
arrays replace the entire value. This is a narrow ADR-002 partial-update
convention for this route, not an implicit JSON Merge Patch or JSON Patch.
Unknown/tombstoned/system field IDs are rejected; createdBy, times, id and
recordVersion are never writable. Values returned by GET/search include only
the fields readable on that row, with no redacted sentinel or nullable alias
that could reveal a hidden value. System fields remain visible only after row
authorization. Reads through two views of the same table observe one row and
version. A not-yet-Saved V030-013 table (schemaReady=false) rejects record
operations with 409 APPLICATION_SCHEMA_NOT_READY, rather than manufacturing
a JSONB fallback. GET for a record that is not currently readable, or whose
route crosses app/table/view ownership, returns 404; lacking the actual form
entry returns 403.

The request operationId uses the shared actor/key namespace. Record write
results and durable operation results contain only MutationResult, never
business field values. The currently authenticated actor can query their own
minimum confirmation result even after data permission is revoked; the
operation endpoint never returns old field data. Fingerprint binds
method, operation kind, actual route app/view/table, normalized complete body,
expected versions, queryVersion if supplied, and draftRef. QueryVersion is a
snapshot/interaction guard, **not** permission evidence; current permission
is reloaded. It is optional for direct detail editing, while list-originated
frontend edits must supply it. Confirmed replay uses the original minimum
result after live actor authentication, before current data CAS/schema checks.
A response/COMMIT
loss preserves the original key and exact body and directs the client to GET
the existing operation; 404 or timeout does not prove rollback. No automatic
new-key retry. Draft saves have no record/audit/flow side effects but their
own operation result makes ambiguous commits recoverable.
Draft operation results store only DraftMutationResult or the minimum discard
confirmation, not payload values. A replay cannot re-expose a draft field
after its permission has been revoked.

DraftCreate initializes values, and PATCH changes only listed keys. Keys in
changes and removeFieldIds cannot overlap; duplicate removals reject. Omitted
keys remain byte-for-byte the saved canonical draft payload. removeFieldIds
can explicitly clear an old key from the actor's own draft even when that
field is now removed or no longer writable, without returning its old value.
New/changed values require current field create/edit permission. Empty changes
and empty removals preserve draftVersion and return a durable idempotent
minimum result. A draft may omit required fields or contain unfinished
business values: draft save checks field IDs, live permission, JSON shape,
Unicode and raw 1 MiB request length; it does not run complete V030-013
value/reference/required validation until formal submission.

GET Draft returns only fields the current actor can still edit for its
bound create/edit action, plus explicit conflicts without old values.
Summary.hasConflicts is true when any conflict exists. Removed or revoked
fields stay in persistent draft payload until the owner explicitly removes
them; submission blocks while such keys remain. Full form-resource permission
loss is 403, not a draft-read bypass. Schema/base-record changes preserve
the original bound versions and draft, return SCHEMA_CHANGED or
BASE_RECORD_CHANGED with fieldId=null, and block submission. No automatic
rebase or deletion occurs. A user may explicitly save a **new** draft against
current versions through the normal create route; this version has no
rebase API.

Filtering semantics frozen in ADR §8.6: root AND/OR
group, nested groups, leaf value typed by the current field definition.
Text/multiline permit eq/neq only; decimal and date/datetime permit
eq/neq/gt/gte/lt/lte with exact numeric and UTC semantics;
boolean/single_select/member/department permit eq/neq only. Multi-select
eq/neq use normalized duplicate-free set equality/inequality, never contains;
array order is not significant. Empty optional multi-select normalizes to
NULL under V030-013, so [] follows the null comparison rule. Non-null neq
excludes NULL; eq null means IS NULL, neq
null means IS NOT NULL. Option/ref predicates compare stable IDs; never labels
or stale display. Text sorting is absent. Default order is createdAt DESC,
id DESC; explicit numeric/time sort appends id in the same direction, NULLS
LAST. Offset is (page-1)*pageSize with checked safe arithmetic; any valid page
number may be requested, including one beyond total (empty items, no silent
clamp). PageSize is 1..100; groups are at most 3 levels/20 leaves; raw search
body is at most 65536 UTF-8 bytes including wrapper and canonical filter at
most 16384 bytes. Raw record/draft write bodies are at most 1 MiB each.
HiddenColumnIds remains visual state and never changes server
filter/sort authorization. Relative dates, if exposed later, must be frozen
to an absolute range in query context before first page.

Data grants name the concrete form view; the same record still lives in
its one physical table. A data.create grant may have an empty field mask,
allowing a defaults-only create. Every client-supplied field must be covered
by a matching create mask; server defaults still pass current normalization
and required checks. The complete projection includes **all currently
authorized fields of the table for that form**, even if layout omits a field
or the browser hides a column, plus all observable reference display.
Incomplete drafts use independent DraftValues and are not falsely normalized
as final FieldValue. Quick search remains a separate proposal below; ordinary
text filter remains exact eq/neq.
Accepted ADR-002 API-13 defaults to pageToken pagination; its Q25 page/pageSize
exception is expressly limited to personnel endpoints. Its PR21 read-only
POST/search exception is likewise limited to personnel members/events.
V030-015 ADR §8.3 records **new limited ADR-002 exceptions** solely for
record search. Draft GET remains
cursor-paginated under the accepted default.

### Quick-search proposal for a separate review

The original table shows a search control, but its searchable fields and
matching rule are not frozen. Proposed additive request member, **not
selected for implementation until lead review**:

    type QuickSearch = {term:string; fieldIds:UUID[];
      match:"contains_literal"} | null;
    type RecordSearch = {page:Version; pageSize:Version;
      filter:FilterGroup|null; sort:Sort; quickSearch?:QuickSearch;
      queryVersion?:string};

The explicit fieldIds must be nonempty, distinct current text/multiline
fields of this table. Each must be readable across the entire visible row
scope. Match is an OR of literal case-sensitive substring tests over those
fields, with term bound as data and LIKE metacharacters escaped. No implicit
scan of every field, reference display, hidden column, system field, or
unauthorized value. Empty term is equivalent to null/no condition. Literal
contains can scan N rows; pg_trgm or another index is a separate measured
migration choice. REVIEW: approve these exact scope/case rules or supply
the intended Figma search semantics; until then there is no backend
quick-search contract.

Proposed additive error mapping (names/statuses for review, not registered):

| Code | HTTP | data / meaning |
| --- | --- | --- |
| COMMON_VALIDATION_FAILED | 400 | existing body-field error format |
| APPLICATION_FORBIDDEN | 403 | no actual resource entry/action/field permission; no sensitive values |
| APPLICATION_NOT_FOUND | 404 | unreadable/cross-app record or actual app/view ownership mismatch |
| APPLICATION_RESOURCE_INVALID | 400 | invalid input field/ref or cross-app reference |
| APPLICATION_SCHEMA_NOT_READY | 409 | form table has not completed first definition Save |
| APPLICATION_SCHEMA_CONFLICT | 409 | currentSchemaVersion for authorized actor |
| APPLICATION_RECORD_CONFLICT | 409 | currentRecordVersion for authorized reader only |
| APPLICATION_RECORD_FENCED | 409 | pending approval command or protected write |
| APPLICATION_QUERY_CHANGED | 409 | complete authorized result changed; explicit page-one refresh |
| APPLICATION_QUERY_CONTEXT_EXPIRED | 409 | missing/evicted/invalid context; no claim of data change |
| APPLICATION_DRAFT_CONFLICT | 409 | draftVersion mismatch, no silent overwrite |
| APPLICATION_DRAFT_BASE_CONFLICT | 409 | base record/schema changed; preserve draft |
| APPLICATION_OPERATION_CONFLICT | 409 | same actor/key with different fingerprint |
| APPLICATION_OPERATION_UNCONFIRMED | 503 | original operationId; reconcile |
| COMMON_SERVICE_UNAVAILABLE | 503 | required guard/source/registry unavailable or precommit timeout |

ADR §8.8 froze these codes; shared registrations/details remain with V030-013 owner. GET of an
unreadable/cross-app record is 404, while absent form entry is 403.
No record DELETE, restoration, import/export, approval/reversal/complete or
notification route is proposed. Those product policies are not frozen.

## 3. Action/row/field authorization matrix

All checks use the actual same-app resource registered by V030-013, live
Session principal and effective group membership. Bootstrap and immutable
application owner have full supported actions only for real existing resources;
the separate applications.create capability conveys no rights in another app.
Ordinary group grants form indivisible (resource,action,rowScope,fieldMask)
tuples. Enabled matching tuples are ORed; missing checkbox is no grant, not
DENY. No subordinate, directory inheritance or cross-app grant merging.

| Operation | Resource/action | Row condition | Field condition | Additional gate |
| --- | --- | --- | --- | --- |
| Search/count/get | exact form, data.read | all OR createdBy=actor | all authorized table fields, regardless of view layout or hidden columns | live menu entry separately; schemaReady |
| Filter/sort | exact form, data.read | entire visible all/own row scope | every referenced field grant must cover that whole scope or request 403 before reading its values | server authorization before filter, COUNT, LIMIT |
| Create | exact form, data.create | new createdBy=actor; all-only action grant | client fields in matching create mask; empty mask permits defaults-only | defaults/required/reference validation |
| Edit | exact form, data.edit | all OR immutable createdBy=actor | every changed field in matching edit masks | recordVersion CAS and mandatory fence |
| Draft save/list/get | exact form and owner from Session | draft.owner_user_id=actor | save checks current create/edit mask; read omits now-inaccessible values without modifying stored draft | schema/base version, draft CAS/conflict state |
| Approval-node Save later | same RecordWriter with separate task-scoped authorizer | actual task actor and task state verified by future flow contract | node-specific field whitelist, independent of ordinary data.edit | pending-command fence and CAS; no auto-complete |

For any record r and field f, authorization is:

    can(r, action, f) =
      real_resource(r) AND live_actor
      AND (trusted_bootstrap OR actor = immutable_app_owner
           OR EXISTS enabled_same_app_grant g:
              member(actor,g.group) AND g.resource=actual_form
              AND g.action=action
              AND (g.scope=all OR (g.scope=own AND r.created_by=actor))
              AND f IN g.field_mask)

Never precompute independent sets of actions, scopes and fields and take their
Cartesian product. An own-edit-X grant plus a read-Y grant must not edit any
other row or field. For SQL, construct a grant relation tied by group/grant ID,
then EXISTS by action + resource + row predicate + field ID. Data.create
also needs an action-level authorization method because an explicit empty
mask is a valid defaults-only grant; field-level Allows alone cannot express
it. A row without any readable business field is excluded from COUNT/page.

Before reading field values, derive the visible row scope from effective
same-form data.read tuples with nonempty masks: all if any all grant exists,
otherwise own if any own grant exists, otherwise no rows. For each requested
filter/sort/quick-search field, derive its scope from matching complete
data.read tuples. Visible all requires field all; visible own is covered by
field own or all. If any field does not cover the full visible scope, reject
the entire search with 403 before evaluating restricted values. For example,
read-X-all plus read-Y-own does not allow sorting all visible rows by Y.
No subset of rows is silently excluded to make a filter appear authorized.
Owner/Bootstrap covers all only after real form and table existence checks.
The exact scope predicate is compiled into the SQL authorization stage
before user filter, COUNT and page.

Write authorization, required/default normalization, schemaVersion and
recordVersion CAS, draft consumption and fence are checked in the final write
transaction under locks. Client-hidden columns confer no authority. Policy
changes must invalidate or re-evaluate query contexts before result release,
even when data revision did not change.

## 4. Caller-owned transaction and fence ports

V030-013 authoritative ADR fixes the PR21/B5 order: transaction-local limits;
personnel.lock_query_revisions(); AccessForWrite and source locks sorted by UUID;
app/policy gate and operation claim; app structure then sorted table gate;
physical value/DDL lock; values/metadata/audit/result; one commit. V030-015
record writers must take the **same table gate before touching rows**, then row
lock/CAS and mandatory fence. They cannot use BeginManagerWrite for ordinary
members because it requires owner/Bootstrap. V030-013 must supply or approve
a shared lifecycle variant whose caller supplies an action policy, while
retaining one actor+operationId ledger and unknown-COMMIT classification.

Proposed full integration boundaries (exclusive packages now have partial
caller-owned primitives; these shared lifecycle ports do not yet exist):

    type TrustedRecordContext struct {
      ActorID, AppID, TableID, ViewID string
      SchemaVersion, PolicyRevision int64
      // Constructed only after Session/resource/live-policy reads in tx.
    }
    type RecordFence interface {
      CheckWrite(ctx context.Context, tx pgx.Tx,
        trusted TrustedRecordContext, recordID string,
        expectedVersion int64, intent WriteIntent) error
      // Authoritative local pending-command table; pending => 409.
      // Missing/unavailable implementation => 503. A real empty table is valid.
    }
    type RecordActionAuthorizer interface {
      CheckOrdinaryCreateOrEdit(ctx context.Context, tx pgx.Tx,
        trusted TrustedRecordContext, action string,
        createdBy string, changedFieldIDs []string) error
      // Exact data.create or data.edit complete grant tuples.
    }
    type TaskSaveAuthorizer interface {
      CheckTaskSave(ctx context.Context, tx pgx.Tx,
        trusted TrustedRecordContext, task TaskContext,
        changedFieldIDs []string) error
      // Future task owner supplies real task state/actor/node whitelist.
      // Does not require ordinary data.edit unless flow contract later says so.
    }
    type RecordWriteAuthorization interface {
      Check(ctx context.Context, tx pgx.Tx,
        trusted TrustedRecordContext,
        request NormalizedRecordWrite) error
      // Concrete ordinary or task-scoped adapter; nil always fails closed.
    }
    type ReferenceActivity interface {
      CheckNewValue(ctx context.Context, tx pgx.Tx,
        fieldID, kind, sourceID string) error
      // Active authoritative source only; absent => 503.
    }
    type RecordChangeWriter interface {
      AppendInTx(ctx context.Context, tx pgx.Tx,
        trusted TrustedRecordContext, request NormalizedRecordWrite,
        before, after CanonicalFieldValues,
        result MutationResult) error
      // Canonical changed fields only, stored in restricted app history.
    }
    type RecordWriter interface {
      ApplyInTx(ctx context.Context, tx pgx.Tx,
        trusted TrustedRecordContext, request NormalizedRecordWrite,
        authorization RecordWriteAuthorization,
        fence RecordFence, refs ReferenceActivity,
        history RecordChangeWriter) (MutationResult,error)
      // Never Begin/Commit/Rollback, never remote Flowable call.
    }
    type DraftConsumer interface {
      ConsumeInTx(ctx context.Context, tx pgx.Tx,
        ownerID, appID, tableID, viewID string,
        ref DraftRef, baseRecordVersion int64) error
      // Exact owner/resource/schema/base/draftVersion; same tx as record.
    }

The shared owner should expose BeginRecordWrite/Commit/Replay/Claim/Complete
or an equivalent caller-owned lifecycle. Replay requires current result
readability limited to the authenticated actor's minimum confirmation, but
is checked before stale CAS, token or draft consumption;
otherwise committed response loss cannot recover. A draft submitted to a
record create/edit is consumed only when all exact bindings match and the
record+audit+operation transaction commits. A newer saved draft survives
the older version's submit; absent/mismatched draft is a conflict, not a
silent ignored ref. Pure draft save uses owner-scoped auth/CAS but no business
table revision, record audit, process trigger or record fence. Draft lists
use indexed owner/page queries rather than reading every draft into memory.
A saved draft may omit unfinished fields and preserve incomplete business
values. Commit always performs full current business validation.

For future node Save, the caller must inject a real task-scoped authorizer
with task actor/state and node field whitelist, **separate from ordinary
data.edit authorization**, plus the same authoritative pending-command
fence and expected recordVersion. The flow owner must freeze its task rules.
Save changes the original record and change history in one PG transaction
and does not call complete/agree, launch other workflows or make a remote
Flowable request while holding PG locks. Cross-service approval protocol
remains separately undecided. An available, authoritative local fence
table with no pending row correctly allows a write. A hardcoded no-op or
missing guard is forbidden.

## 5. Selected query algorithm and retained alternative

Define one canonical complete authorized projection P for a query context:
ordered matching record IDs (stable ID tie), COUNT, all currently authorized
table field values for this concrete form independently of layout and
hiddenColumnIds, and every observable reference display for matching rows.
Its criteria include the actual app/table/view, normalized
filter/sort and frozen date bounds; policy/schema versions and registry source
revision are separate observed dependencies. A change is relevant iff P
changes under the same current authorization. Policy loss is an immediate
authorization error, not permission to replay stale P. Context expiration is
separate from QUERY_CHANGED. A changed matching row outside the current
page is relevant; an unchanged/no-op write, an unrelated row, and an
unreadable field outside P are not. The lead selected this projection boundary;
formal Notion readback still gates code.

Let N=table rows, M=matching authorized rows, F=fields/leaves evaluated,
W=bytes of one projected row, p=page size, o=(page-1)*p, C=active contexts,
E=events since a context's cursor, D=distinct changed rows, H=retained
events, and B=server-side bounded stream batch. SQL scan/sort costs depend
on indexes/selectivity; the expressions below are upper-order models, not
latency promises. PostgreSQL sort/hash can spill to disk; process RSS is
not the whole query memory.

### Selected A: generalize the existing complete-projection engine

Keep Q36 lifecycle and Redis CAS/Session binding; extract reusable criteria
normalization, context metadata, revision-vs-fingerprint decision, RR
snapshot, bounded streaming hash and page-one refresh semantics. Keep
personnel SQL vocabulary isolated; appquery uses metadata-compiled typed
SQL and the same shared lifecycle, not a second web table adapter. Existing
personnel functions are package-private, so one coordinated owner must
extract the lifecycle. Duplicating the Redis context implementation inside
appquery is not reuse. On a data,
policy, schema or relevant registry revision change, stream the complete
authorized projection in the **same RR snapshot** and compare digest+count.
Only a difference returns QUERY_CHANGED. If equal, advance context
revision using Redis CAS. If revision unchanged, reuse the saved total and
run the authorized page-only query after live actor/policy checks. For a
new/changed query, build
its complete fingerprint before returning a context. Frozen filter, exact
auth predicate and reference display are part of the digest oracle.

Cost: on invalidation check O(scan(N,F)+sort(M)+M*W) DB work and O(M*W)
streaming bytes, O(B*W+p*W+F) application memory with bounded fetch batches;
PostgreSQL sort can need
O(M) work memory/temp files. If no revision changed, page work is
O(index seek + o + p) for a usable ordered index and can approach O(N log N)
with unindexed dynamic sort; COUNT is O(index/candidate scan), never
constant in general. Initial query has the full-projection cost. Context
metadata O(C*(criteria bytes+constant hash/version)), not O(C*M).
One unrelated write bumps the table revision and may force O(C) complete
validation work across C active contexts when each next used; user-visible
refresh still occurs only for changed P. No event-retention growth or
old-value duplication. Initial and invalidated full projection is a cost to
measure at 10k/100k/1m and 1/20/200 contexts. If it is unacceptable, return
plans/RSS/timing evidence to the lead; increasing timeouts alone does not
change the selected design.

Correctness proof obligation: digest includes every observable authorized
projection byte and selected source display, deterministic order and schema
version. A cryptographic collision remains theoretical; independent tests
compare exact expected result. Any source hook missing a revision can create
false negatives, so unavailable source integration must fail closed.

### Unselected B: durable bounded event journal with exact old/new impact

Retained as comparison evidence only. Do not implement its persistent log,
old-value duplication, migration or retention policy in this release.

Under the same table gate and transaction as each record write, assign a
monotonic per-table revision and append a row-change event before commit.
Record events need row ID, changed field IDs and **old/new typed values
needed to reconstruct filter, sort, visible values and references**; an
ID/field-mask-only log cannot detect a row that left a filter or determine
past sort position. Sensitive old values in a journal create a second
protected retention/backup surface. A design that evaluates every active
context at write time instead costs O(C*F) per write and is not assumed.
Source display and policy changes require their own versioned events or a
full-projection fallback. Table gate ordering makes committed revisions
contiguous; context cursor is read in the same RR snapshot as P/page.

At validation, stream events (cursor,current] under RR. For each distinct
changed row, reconstruct its before/after state from ordered deltas and the
current typed row, evaluate the saved authorized filter and projection on
both, and compare membership/order key/visible values/reference display.
Any actual difference in P returns QUERY_CHANGED; otherwise CAS-advance
the context cursor. A policy/schema change, journal gap or insufficient
source event detail invokes Candidate A full projection, **not** automatic
refresh or a guessed unchanged result. Pruning policy is not set here;
once a context cursor predates the retained floor, full scan is mandatory.
Journal event insertion/retention cannot be outside the record transaction.

Cost: writes add O(changed-value bytes) event I/O and indexes and retain
O(H*average-delta-bytes) PG storage plus WAL/backup; if a source change
fans out to R referencing records, checking relevance can cost O(R) index
lookups/reads unless an exact per-query summary is maintained. Read
validation O(E*F + D*row-fetch + reference checks), streaming memory
O(B*(delta bytes+row width)+p*W+F); event sorting/reconstruction can use
PG temp space. Worst E≈all writes, and a gap/policy/schema change falls
back to Candidate A. Active Redis context metadata O(C*criteria), not
O(C*M). Multiple events on one row and changed field value reconstruction
must be tested; journal row ID alone is insufficient. Source and policy
events can have much greater fanout than record edits. Deep page still
pays OFFSET/sort/COUNT costs. Restrict journal access to the minimum role,
exclude raw values from logs, and obtain explicit retention/backup policy
before selecting this algorithm.

### Selected tradeoff

| Property | A full projection | B event journal + fallback |
| --- | --- | --- |
| Unrelated write and one active context | full rehash, no refresh if equal | inspect delta, no refresh if equal |
| Related write | full rehash then changed | delta comparison then changed |
| Write amplification | revision only | delta row/index/WAL, source fanout |
| Persistent growth | context metadata only | H events/old values until approved prune |
| Authorization/schema change | full rehash/reject | full fallback/reject |
| Proof surface | exact deterministic projection + all writers bump revision | delta completeness, reconstruction, gaps, source/policy fanout plus fallback |
| Main risk | high repeated reads at N=1m and C contexts | sensitive duplicate values, retention, missed event, write load |

The lead selected A for this version because it uses the reviewed Q36
correctness pattern without a second old-value query journal. This does
not claim million-row performance has passed. Neither
algorithm can promise a constant-time arbitrary page or avoid current
permission checks. No complete result is sent to or held by the browser.

## 6. SQL, reference and migration proposal

Query compiler resolves every fieldId via one V030-013 table metadata snapshot,
uses internally generated physical f_<uuid> column identifiers and typed binds
only, and rejects tombstoned/mismatched IDs. First, in the same actor/policy
snapshot, check that each filter/sort field's all/own grant coverage contains
the entire visible all/own row scope. Failure is 403 before any restricted
field value read. Then the SQL shape is:

    WITH grants AS (effective complete tuples for current actor/app/view),
    authorized AS (
      SELECT r.* FROM appdata.t_<table_uuid> r
      WHERE EXISTS (matching data.read field grant for this row)
    ),
    matched AS (
      SELECT ... FROM authorized r
      [registry joins/EXISTS required by reference predicates]
      WHERE compiled_filter
    )
    SELECT count(*) FROM matched;
    SELECT ... FROM matched ORDER BY sort_key NULLS LAST,id LIMIT $p OFFSET $o;

This is a logical shape, not a mandate to materialize all rows. COUNT/page
and grant coverage must share one RR snapshot; actual SQL must use
permission-conditioned predicates where required to prevent optimizer
reordering from evaluating restricted values on unauthorized rows. EXPLAIN
chooses the physical
plan without weakening that rule. No unauthorized values are sent to Go for
filtering, sorting or full-table client evaluation. Field masking is
performed per returned row after SQL authorization, before serialization;
unauthorized references are never resolved for display. Test that COUNT
and sort cannot leak an unpermitted field. Hidden columns do not narrow
grants.

No-reference-predicate path: COUNT and page IDs first, then resolve distinct
page refs in batches in that same RR transaction, with normalized member to
multiple-department relation; nullable remains null and missing registry is an
integrity/unavailable error, not empty display. Reference predicate path:
join/EXISTS authoritative registry and member-department relation before
COUNT and LIMIT. Source deletion leaves stable ID and tombstone last display;
new selections need active source. Department rename/move and member
deletion must use real personnel source/version hooks; do not clear old
references or choose a first department. If B4a's scalar deptRef contract
cannot represent many departments, extend the integration registry with
member_department edges rather than silently flattening source semantics.

Proposed indexes after actual plans and limits review:

- Each physical table: id PK (V030-013), (created_by,id) for own grants;
  (created_at DESC,id DESC) for default order; one reviewed btree
  (f_<sortable>,id) per approved numeric/date/datetime sort field where
  workload justifies write cost; scalar UUID reference btree for equality;
  GIN on multi_select uuid[] only if filtered and measured. Text equality
  btree may help; neq/OR/low-selectivity still scan. Dynamic index count
  and synchronous CREATE cost must be approved with schema Save policy.
- Grant lookup: (app_id,group_id,resource_kind,resource_id,action,row_scope)
  with FK to real registered resource; grant_fields (app_id,grant_id,field_id);
  group_members (user_id,app_id,group_id) already exists. Revised constraints
  must accept exact data actions and prevent cross-app/field tombstone grants.
- Selected A stores only revision/fingerprint query metadata; no query
  event-journal index or old-value log is created. Reverse reference lookup
  for source fanout still needs measurement; use typed field indexes or a
  maintained relation only after contract approval, not an unbounded table
  scan hidden in a hook.
- Drafts: (owner_user_id,updated_at DESC,id DESC) plus unique id; composite
  app/table/view/target checks, schema/base/draftVersion constraints. Never
  store Session secrets or grant readable drafts to other owners.

If V030-013's hot7/cold4 land unoccupied, next candidate slots are hot8 for
record policy constraints, grant fields, draft/revision and restricted
source mapping and hot restricted record history. Cold5 is only a candidate
if the lead approves an archive policy. Recheck actual merged
HEAD and owner allocation before assigning filenames. Preserve hot1–7 and
cold1–4 bytes. Physical business rows remain typed; JSONB is acceptable only
for metadata, draft payload or restricted history deltas. Draft payload is
not a committed record. The B5 operation table currently permits only 200/201
results and five operation kinds; frozen draft-discard 204 requires an
additive operation kind/status constraint and an internal durable
result object, while the external 204 has no body. Record change history
may retain controlled old/new values only under the separate privacy
contract below; no such values go to globally visible auth audit.
Hot/cold role grants must be explicit and
least privilege; V030-015 does not edit shared roles or migration files now.

### Precise handoff to V030-013 integration owner

- V030-013 must finish the authoritative field normalizer, active field
  lookup, form-to-table registry, schemaReady, same-table gate and
  caller-owned structure/operation transaction ports. At inspected
  269c774, appschema.Save still self-commits; the ADR's ApplyInTx is a
  frozen target, not a callable delivered method.
- Extend applications resource registration for concrete form views while
  preserving the existing app-root menu entry. Extend applications.grants
  action CHECK to data.create/data.read/data.edit, scope CHECK to all/own
  for record actions (create uses all), and grant_fields with actual
  same-app form/table field FK or equivalent protected registry validation.
  A data.create grant with zero field rows is valid action permission.
  No grant can point to a removed/tombstoned or other-app field.
  Persist each complete tuple and read its field rows without crossing
  group/action/scope boundaries. apppolicy needs a coordinated
  action-level create check plus existing cell checks; the current pure
  evaluator and B5 stored snapshot cannot authorize these writes yet.
- Provide a non-manager BeginRecordWrite lifecycle under current Session:
  PR21 revision locks, live AccessForWrite, app/resource/policy locks,
  operation replay/claim, V030-013 table gate, then row/CAS/fence, typed
  normalization, restricted change history, draft consume and minimum
  operation completion, with one commit/unknown classification. Existing
  B5 actor+operationId uniqueness must cover record and draft kinds.
  Record/draft result JSON is minimal; draft-discard 204 is an additive
  ledger status and response exception.
- Extract Q36's RR query-context/Redis CAS lifecycle into one shared
  backend owner boundary usable by personnel and appquery, preserving
  personnel endpoint behavior and existing context keys. The dynamic
  typed compiler stays in appquery; the existing Table interaction uses
  the same queryVersion/explicit-refresh shape. Coordinate edits to
  personnel/query_engine.go and query_context.go; copying them into
  appquery is not acceptable.
- Coordinate hot8/cold migration allocation and least-privilege roles
  after V030-013 hot7/cold4 land. Hot8 candidates: data grants/fields,
  record revision, drafts, restricted record history and source relation;
  **no query event journal**. Cold5 is reserved only if a separate
  archive policy is approved. OpenAPI/errors/BFF composition remain the
  V030-013 owner's shared files, with this document as the delta.
- The Flowable owner must supply an authoritative local pending-command
  fence and later task-scoped Save authorization. An available empty
  fence table is valid; absence/unavailability is 503, not a no-op.
  B4a integration must supply real personnel hooks/source versions,
  display mapping and normalized multi-department edges; its local
  module alone is insufficient.

### Separate record change history: privacy/retention proposal for freeze

Ordinary edits and future financial-node Save must preserve enough canonical
before/after data to inspect a changed amount. Propose two application-owned
hot tables, distinct from personnel-visible auth.authentication_events:

    record_change_events {
      id UUID PK; app_id UUID; table_id UUID; record_id UUID;
      source_view_id UUID; actor_user_id UUID; operation_id UUID;
      record_version_before BIGINT; record_version_after BIGINT;
      origin "ordinary"|"task_save"; opaque_task_ref TEXT|null;
      occurred_at TIMESTAMPTZ
    }
    record_change_values {
      event_id UUID FK; field_id UUID; field_kind TEXT;
      old_value JSONB NOT NULL; new_value JSONB NOT NULL;
      PRIMARY KEY(event_id,field_id)
    }

JSONB here is a **restricted history delta**, not the typed business row.
old_value/new_value are canonical wire scalars/arrays or JSON null; the
field_kind is the kind at the write version. Changed fields only, no full
record snapshot, credentials, Session, raw request body or unrelated fields.
The event and each delta are inserted inside RecordWriter.ApplyInTx with
the typed row, draft consumption and minimum operation result. A no-op
may produce a header-only operation audit but no value delta or version bump.
One committed value-changing recordVersion has one event; operationId and
record identity are unique together to prevent duplicate history on replay.
Cold archive/retention dates, privacy classification and deletion behavior
are **not** invented here; no automatic purge or retention period is proposed.
Backups of this table contain potentially sensitive history and require the
same restricted role/encryption controls as business data.

Proposed read route, not part of the current OpenAPI until freeze:

    GET /forms/{viewId}/records/{recordId}/history?pageSize=&pageToken=
    type HistoryChange = {fieldId:UUID; fieldKind:string;
      before:FieldValue; after:FieldValue};
    type HistoryEvent = {id:UUID; recordVersionBefore:Version;
      recordVersionAfter:Version; actorId:UUID; occurredAt:string;
      origin:"ordinary"|"task_save"; changes:HistoryChange[]};
    // Response data.items:HistoryEvent[]; meta.pagination: ADR-002 cursor.

Use the *current* actual form, row createdBy and current data.read grants
to authorize the row and each field before projecting history. A caller
without current row read gets 404. A change event with no currently
readable changed fields is omitted before LIMIT and cursor advancement,
so actor/time/version cannot reveal a hidden-only edit. A mixed event
returns only authorized field deltas; it never reports the count/IDs of
hidden changes. Permission revocation takes effect on the next history
read, including old pages and operation replay; neither owner/Bootstrap
can bypass real resource existence. Cursor is scoped to actor/session,
form/record and current policy/schema revision; stale cursor is rejected
or revalidated before release. The historical source view is provenance,
not an alternative authorization path. Source UUID values remain stable;
display resolution at history-read time uses current registry/tombstone and
must not pretend to be an immutable old-name snapshot.

REVIEW before implementation: whether current data.read alone is the
history read capability or whether a separate history action is needed;
whether actor identity/time in mixed visible events is approved; the
retention/archive/backup and sensitive-field policy; and whether task
references may be returned at all. Until frozen, no history values go in
the global authentication audit summary, logs, operations result or
unprotected cold archive.

## 7. Independent RED/GREEN and scale acceptance after release

The isolated core subset in [core-red-green.md](core-red-green.md) ran; the
full acceptance matrix below remains **NOT RUN**. Required evidence: source revision, isolated
PG18.6 and restricted auth_app role, exact independent expected rows/values,
initial target RED before code, committed GREEN, command/output/hash/time and
unrun/fail/skip distinction. Mock-only tests cannot certify these contracts.

| Matrix | Cases / oracle |
| --- | --- |
| Auth | two actors/two apps/two forms, owner/Bootstrap trusted flag spoof, create-only permission including empty-mask defaults-only, no inherited child resource, exact own createdBy after editor changes, own-edit-X + other-read-Y cross-product, menu vs data, revoke/write races, read-X-all plus read-Y-own filter/sort 403 before value evaluation, zero-readable-field row excluded, GET unreadable record 404 |
| Typed records | every V030-013 field kind/default/null/decimal/time normalization, no JSONB business row, stable IDs, shared-table multi-view, schemaReady=false, field tombstone/unknown ID, required/option/ref rejection, CAS two writers, no-op |
| Transaction | typed row+restricted old/new history+minimum operation/optional draft rollback at every fault point, concurrent same operation key, committed response loss and original-key recovery after data-grant revoke without old values, unknown PG commit error, no new-key retry |
| Fence/flow port | missing/unavailable fence fails closed; real empty fence table permits ordinary write; pending approval command blocks edit; future node Save has separate task-scoped actor/field authority, CAS/change history same tx and no automatic complete/other flow trigger |
| Query correctness | nested AND/OR, NULL eq/neq, text eq/neq only, multi-select dedup set eq/neq not contains, decimal/date/datetime compare/order, duplicate/NULL refs, equal sort ID tie, page 1/deep/after-end, full authorized COUNT, layout/hidden-column independence |
| Relevance | nonmatching row/field no refresh; entering/leaving filter, off-page visible edit, reorder, count change, source rename/delete/tombstone, policy grant/revoke, schema change, context eviction, concurrent snapshot/writer race |
| Draft | owner Session, cross-user/app/view denial; independent incomplete DraftValues; PATCH partial changes/removals, overlap rejection and empty no-version-bump; schema/base/draft CAS; field revocation masked GET with conflict and persistent old key until explicit removal; whole-resource loss 403; no automatic rebase/deletion; same-tx consumption and ambiguous commit/replay |
| History privacy | current row/field grant masks old/new by field; hidden-only event omitted before cursor/LIMIT; mixed event exposes only permitted changes; revoke after prior read blocks next read; auth global event contains no values; ordinary and task Save both record exact delta; no fixed purge |
| Sources | real personnel many-department move, department rename, member delete, same-name new account, late source version, registry missing, references selected only while active, predicate before COUNT/LIMIT under same RR |
| Capacity | N=10k/100k/1m; selective/broad/own grants; 0/1/20 leaves and AND/OR; p=20/100 and o near 0/50k/500k; 1/20/200 active contexts; low/high edit rate; source fanout 1/1k/100k; A initial/unchanged/related/unrelated-revision rehash |
| Operations | EXPLAIN (ANALYZE,BUFFERS), full projection/COUNT/page separately, warm/cold cache, index and temp bytes, WAL/write amplification from restricted history, DB/app RSS peak and PG temp spill, p50/p95/p99 with repeated runs, lock wait/deadlocks, plan after ANALYZE |
| End to end | real HTTPS Session/CSRF, OpenAPI response validation, original Table queryVersion/refresh behavior, Chromium/Firefox/WebKit against actual PG API; no fixture-only completion claim |

No performance budget, query-journal retention or record deletion/
restoration strategy is asserted here. The lead selected A and the stated
wire bounds, but only repeated real benchmarks can establish performance.
A real B4a source-hook artifact and a real
pending-command fence remain integration blockers for those behaviors;
they do not block this review document.

## 8. Remaining integration and separate reviews

The V030-015 PRD was read back frozen and ADR §8 accepted on 2026-10-03.
ADR §8.3 records the narrow record-search exception to ADR-002. This
authorizes the core work in the exclusive packages; it does not make the
routes available until shared integration and acceptance are complete.

1. Connect V030-015's isolated core ports to V030-013's live field normalizer,
   form/table registry, table gate and Session/policy transaction. The current
   branch has no ready HTTP route, persistent data grants, operation extension,
   query-context reuse or product migration. Its PG tests use temporary tables
   and test adapters; they do not prove runtime-role permission or live API.
2. Review the separate quick-search DTO/scope/case proposal before any
   search-control backend implementation. Confirm exact error registrations
   and any current policy/schema change distinction relative to
   QUERY_CHANGED/CONTEXT_EXPIRED.
3. Freeze the restricted record history old/new schema, current-grant
   read rule, actor/time exposure and retention/archive/backup policy
   before old values are stored or exposed. No guessed global audit summary.
4. Assign shared applications transaction lifecycle, apppolicy data.create
   and grant storage extension, Q36 query-context extraction, OpenAPI/errors,
   hot/cold migration numbers and roles to the V030-013 integration owner.
   V030-015 cannot write those shared files concurrently.
5. Provide authoritative multi-department source mapping/hooks and
   pending-command fence. Until then, dependent operations fail closed;
   a real empty local fence is allowed, a hardcoded no-op adapter is not.
6. Run the independent 10k/100k/1m and 1/20/200-context matrix for A
   after the complete typed query path is integrated. Report actual full-projection cost, memory, plans
   and spill; if unacceptable, obtain a new lead decision rather than
   raising timeout limits to mask it.
