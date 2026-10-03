# V030-015 record/query/draft incremental contract and algorithm review

Status: **PROPOSAL for lead freeze; no product implementation or API availability**.
Source and branch provenance: [task record](../../tasks/V030-015.md).
The V030-015 PRD is pending review and its ADR proposed. V030-013's separate
authoritative ADR freezes structure/field semantics; this document only extends
those boundaries. Choices labeled DECIDE remain unapproved.

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

## 2. Proposed exact HTTP delta

All routes below are proposals beneath /api/v1/applications/{appId}. Their
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
| POST /forms/{viewId}/records | RecordCreate | Record/201 + Location |
| GET /forms/{viewId}/records/{recordId} | none | Record/200 |
| PUT /forms/{viewId}/records/{recordId} | RecordEdit | Record/200 |
| POST /forms/{viewId}/records/search | RecordSearch | RecordPage/200 |
| POST /forms/{viewId}/drafts | DraftCreate | Draft/201 + Location |
| GET /forms/{viewId}/drafts | pageSize,pageToken query | DraftSummary[]/200 + meta.pagination |
| GET /forms/{viewId}/drafts/{draftId} | none | Draft/200 |
| PUT /forms/{viewId}/drafts/{draftId} | DraftUpdate | Draft/200 |
| DELETE /forms/{viewId}/drafts/{draftId} | operationId,expectedDraftVersion query | 204 |
| GET /api/v1/application-operations/{operationId} | existing route | additive record/draft result kinds |

Proposed wire types, all shown keys required unless suffixed ?:

    type UUID = string; type Version = number; // safe JSON integer 0..2^53-1
    type FieldValue = string | boolean | string[] | null;
    // Exact kind drives validation: decimal is a decimal string; date YYYY-MM-DD;
    // datetime explicit-offset RFC3339 normalized UTC; option/ref stable UUID.
    // Only multi_select uses string[]; null is explicit clear.
    type Values = Record<UUID, FieldValue>;
    type Record = {id:UUID; appId:UUID; tableId:UUID; viewId:UUID;
      createdBy:UUID; createdAt:string; updatedAt:string;
      recordVersion:Version; schemaVersion:Version; values:Values};
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
    type RecordPage = {items:Record[]; total:Version; page:Version;
      pageSize:Version; sort:Sort; queryVersion:string;
      schemaVersion:Version; viewVersion:Version};
    type DraftSummary = {id:UUID; viewId:UUID; tableId:UUID;
      targetRecordId:UUID|null; schemaVersion:Version;
      baseRecordVersion:Version|null; draftVersion:Version;
      createdAt:string; updatedAt:string};
    type Draft = DraftSummary & {values:Values};
    type DraftCreate = {operationId:UUID; targetRecordId:UUID|null;
      schemaVersion:Version; baseRecordVersion:Version|null; values:Values};
    type DraftUpdate = {operationId:UUID; expectedDraftVersion:Version;
      values:Values};
    type DraftDeleteQuery = {operationId:UUID;
      expectedDraftVersion:Version};

Create is sparse: omitted fields use V030-013 normalized constant defaults;
required fields must be non-null after defaults. Edit is an explicit changed-field
map: omitted fields retain values, null clears an optional field, empty changes
is a no-op with an idempotent operation result, minimal operation audit and no
recordVersion bump.
Unknown/tombstoned/system field IDs are rejected; createdBy, times, id and
recordVersion are never writable. Values returned by GET/search include only
the fields readable on that row, with no redacted sentinel or nullable alias
that could reveal a hidden value. System fields remain visible only after row
authorization. Reads through two views of the same table observe one row and
version. A not-yet-Saved V030-013 table (schemaReady=false) rejects record
operations, rather than manufacturing a JSONB fallback.

The request operationId uses the shared actor/key namespace. Fingerprint binds
method, operation kind, actual route app/view/table, normalized complete body,
expected versions, queryVersion if supplied, and draftRef. QueryVersion is a
snapshot/interaction guard, **not** permission evidence; current permission
is reloaded. Confirmed replay uses the original result after live actor and
result-readability checks, before current CAS/schema checks. A response/COMMIT
loss preserves the original key and exact body and directs the client to GET
the existing operation; 404 or timeout does not prove rollback. No automatic
new-key retry. Draft saves have no record/audit/flow side effects but their
own operation result makes ambiguous commits recoverable.

Filtering semantics proposed to preserve Q36 where applicable: root AND/OR
group, nested groups, leaf value typed by the current field definition.
Text/multiline permit eq/neq only; decimal and date/datetime permit
eq/neq/gt/gte/lt/lte with exact numeric and UTC semantics;
boolean/single_select/member/department permit eq/neq only. Multi-select
filter semantics (set equality versus contains) require separate freeze
before enabling. Non-null neq excludes NULL; eq null means IS NULL, neq
null means IS NOT NULL. Option/ref predicates compare stable IDs; never labels
or stale display. Text sorting is absent. Default order is createdAt DESC,
id DESC; explicit numeric/time sort appends id in the same direction, NULLS
LAST. Offset is (page-1)*pageSize with checked safe arithmetic; any valid page
number may be requested, including one beyond total (empty items, no silent
clamp). HiddenColumnIds remains visual state and never changes server
filter/sort authorization. Relative dates, if exposed later, must be frozen
to an absolute range in query context before first page.

DECIDE: pageSize/depth/leaves/body limits (current Q36: 1..100, three group
levels, 20 leaves, 64 KiB raw, 16 KiB canonical filter) are useful compatibility
candidates, not authorized V030-015 limits. DECIDE: whether create grants may
have an empty field mask for a defaults-only create; whether form or underlying
table is the data grant resource; whether all authorized fields or only the
current form's readable fields enter the complete query projection. The
proposal above uses the form as resource and its readable fields. The lead
must freeze these before test or code, without altering the V030-013 shared
table identity. DECIDE: incomplete draft payload validation and whether the
Figma quick-search control needs a separately specified text-search predicate;
the stated text filter contract is exact eq/neq, not substring search.
Accepted ADR-002 API-13 defaults to pageToken pagination; its Q25 page/pageSize
exception is expressly limited to personnel endpoints. Its PR21 read-only
POST/search exception is likewise limited to personnel members/events.
V030-015 explicitly needs arbitrary record pages and a JSON filter body, so
the lead must record **new limited ADR-002 exceptions** for the proposed
record search route before OpenAPI/HTTP implementation. Draft GET remains
cursor-paginated under the accepted default.

Proposed additive error mapping (names/statuses for review, not registered):

| Code | HTTP | data / meaning |
| --- | --- | --- |
| COMMON_VALIDATION_FAILED | 400 | existing body-field error format |
| APPLICATION_FORBIDDEN | 403 | no current resource/action/field permission; no sensitive values |
| APPLICATION_NOT_FOUND | 404 | actual app/view/record ownership mismatch; avoid cross-app enumeration |
| APPLICATION_RESOURCE_INVALID | 400 | invalid input field/ref or cross-app reference |
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

DECIDE: exact names/details and whether a wholly unreadable row is 403 or 404.
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
| Search/count/get | exact form, data.read | all OR createdBy=actor | serialize only fields in matching data.read masks | live menu entry separately; schemaReady |
| Filter/sort | exact form, data.read | must be an authorized row | every referenced field readable for that row before predicate/sort | server SQL authorization precedes filter, COUNT, LIMIT |
| Create | exact form, proposed data.create | new createdBy=actor; proposed all-only grant | every client-supplied field in matching create mask | defaults/required/reference validation |
| Edit | exact form, data.edit | all OR immutable createdBy=actor | every changed field in matching edit masks | recordVersion CAS and mandatory fence |
| Draft save/list/get | exact form and owner from Session | draft.owner_user_id=actor | stored draft input checked for writable fields on save and rechecked at commit | schema/base version, draft CAS |
| Approval-node Save later | same record transaction port | actual task actor/field whitelist, independently verified | intersection of node whitelist and record edit ability per frozen workflow policy | live task and pending-command fence; no auto-complete |

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
then EXISTS by action + resource + row predicate + field ID. A row without any
readable field is excluded from count/page under this proposal. A requested
filter/sort field that is unreadable on a particular otherwise visible row
excludes that row before user predicates and count; it never evaluates that
field to decide visibility. This prevents secret-value inference through
filter or sort. DECIDE: whether mixed-field permissions instead reject the
whole query; if selected, reject before returning count/page and test the
non-leakage. Either rule must be identical for all pages and COUNT.

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

Proposed Go boundaries (names for review; no code exists):

    type TrustedRecordContext struct {
      ActorID, AppID, TableID, ViewID string
      SchemaVersion, PolicyRevision int64
      // Constructed only after Session/resource/live-policy reads in tx.
    }
    type RecordFence interface {
      CheckWrite(ctx context.Context, tx pgx.Tx,
        trusted TrustedRecordContext, recordID string,
        expectedVersion int64, intent WriteIntent) error
      // Pending command => RECORD_FENCED; unavailable/absent => 503.
    }
    type ReferenceActivity interface {
      CheckNewValue(ctx context.Context, tx pgx.Tx,
        fieldID, kind, sourceID string) error
      // Active authoritative source only; absent => 503.
    }
    type RecordWriter interface {
      ApplyInTx(ctx context.Context, tx pgx.Tx,
        trusted TrustedRecordContext, request NormalizedRecordWrite,
        fence RecordFence, refs ReferenceActivity) (RecordResult,error)
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
readability, but is checked before stale CAS, token or draft consumption;
otherwise committed response loss cannot recover. A draft submitted to a
record create/edit is consumed only when all exact bindings match and the
record+audit+operation transaction commits. A newer saved draft survives
the older version's submit; absent/mismatched draft is a conflict, not a
silent ignored ref. Pure draft save uses owner-scoped auth/CAS but no business
table revision, record audit, process trigger or record fence. Draft lists
use indexed owner/page queries rather than reading every draft into memory.
A saved draft may omit unfinished fields; supplied values and field IDs still
require a frozen validation rule. Commit always performs full current
business validation.

For future node Save, the caller must inject a real authoritative pending
approval-command fence plus task/actor/field whitelist and expected
recordVersion. Save changes the original record and app audit in one PG
transaction and does not call complete/agree, launch other workflows or make
a remote Flowable request while holding PG locks. Cross-service approval
protocol remains separately undecided. An empty no-op guard is forbidden.

## 5. Query relevance and two candidate algorithms

Define one canonical complete authorized projection P for a query context:
ordered matching record IDs (stable ID tie), COUNT, authorized displayed
field values independently of hiddenColumnIds, and reference display for
matching rows. Its criteria include the actual app/table/view, normalized
filter/sort and frozen date bounds; policy/schema versions and registry source
revision are separate observed dependencies. A change is relevant iff P
changes under the same current authorization. Policy loss is an immediate
authorization error, not permission to replay stale P. Context expiration is
separate from QUERY_CHANGED. A changed matching row outside the current
page is relevant; an unchanged/no-op write, an unrelated row, and an
unreadable field outside P are not. This definition is a proposal for freeze.

Let N=table rows, M=matching authorized rows, F=fields/leaves evaluated,
W=bytes of one projected row, p=page size, o=(page-1)*p, C=active contexts,
E=events since a context's cursor, D=distinct changed rows, H=retained
events, and B=server-side bounded stream batch. SQL scan/sort costs depend
on indexes/selectivity; the expressions below are upper-order models, not
latency promises. PostgreSQL sort/hash can spill to disk; process RSS is
not the whole query memory.

### Candidate A: generalize the existing complete-projection engine

Keep Q36 lifecycle and Redis CAS/Session binding; extract reusable criteria
normalization, context metadata, revision-vs-fingerprint decision, RR
snapshot, bounded streaming hash and page-one refresh semantics. Keep
personnel SQL vocabulary isolated; appquery uses metadata-compiled typed
SQL and the shared lifecycle, not a second web table adapter. On a data,
policy, schema or relevant registry revision change, stream the complete
authorized projection in the **same RR snapshot** and compare digest+count.
Only a difference returns QUERY_CHANGED. If equal, advance context
revision using Redis CAS. If revision unchanged, run only COUNT/page as
required by the frozen context contract. For a new/changed query, build
its complete fingerprint before returning a context. Frozen filter, exact
auth predicate and reference display are part of the digest oracle.

Cost: on invalidation check O(scan(N,F)+sort(M)+M*W) DB work and O(M*W)
streaming bytes, O(B*W+p*W+F) application memory; PostgreSQL sort can need
O(M) work memory/temp files. If no revision changed, page work is
O(index seek + o + p) for a usable ordered index and can approach O(N log N)
with unindexed dynamic sort; COUNT is O(index/candidate scan), never
constant in general. Initial query has the full-projection cost. Context
metadata O(C*(criteria bytes+constant hash/version)), not O(C*M).
One unrelated write bumps the table revision and may force O(C) complete
validation work across C active contexts when each next used; user-visible
refresh still occurs only for changed P. No event-retention growth or
old-value duplication.

Correctness proof obligation: digest includes every observable authorized
projection byte and selected source display, deterministic order and schema
version. A cryptographic collision remains theoretical; independent tests
compare exact expected result. Any source hook missing a revision can create
false negatives, so unavailable source integration must fail closed.

### Candidate B: durable bounded event journal with exact old/new impact

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

### Comparison and recommendation for review

| Property | A full projection | B event journal + fallback |
| --- | --- | --- |
| Unrelated write and one active context | full rehash, no refresh if equal | inspect delta, no refresh if equal |
| Related write | full rehash then changed | delta comparison then changed |
| Write amplification | revision only | delta row/index/WAL, source fanout |
| Persistent growth | context metadata only | H events/old values until approved prune |
| Authorization/schema change | full rehash/reject | full fallback/reject |
| Proof surface | exact deterministic projection + all writers bump revision | delta completeness, reconstruction, gaps, source/policy fanout plus fallback |
| Main risk | high repeated reads at N=1m and C contexts | sensitive duplicate values, retention, missed event, write load |

Recommend A as the **baseline to benchmark**, because it already has a
reviewed personnel correctness pattern and no unapproved journal retention
policy. This is not a choice for implementation: lead must decide A, B or a
reviewed hybrid after the matrix below, with measured C/E/N costs. Neither
algorithm can promise a constant-time arbitrary page or avoid current
permission checks. No complete result is sent to or held by the browser.

## 6. SQL, reference and migration proposal

Query compiler resolves every fieldId via one V030-013 table metadata snapshot,
uses internally generated physical f_<uuid> column identifiers and typed binds
only, and rejects tombstoned/mismatched IDs. SQL shape is:

    WITH grants AS (effective complete tuples for current actor/app/view),
    authorized AS (
      SELECT r.* FROM appdata.t_<table_uuid> r
      WHERE EXISTS (matching data.read field grant for this row)
        AND every requested filter/sort field is readable on this row
    ),
    matched AS (
      SELECT ... FROM authorized r
      [registry joins/EXISTS required by reference predicates]
      WHERE compiled_filter
    )
    SELECT count(*) FROM matched;
    SELECT ... FROM matched ORDER BY sort_key NULLS LAST,id LIMIT $p OFFSET $o;

This is a logical shape, not a mandate to materialize all rows. COUNT/page
must share one RR snapshot; actual SQL must use permission-conditioned
predicates (for example CASE WHEN the exact field grant matches THEN the
typed predicate ELSE FALSE END) so optimizer reordering cannot expose a
restricted value through error or membership. EXPLAIN chooses the physical
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
- Query journal if selected: (table_id,revision) covering exact committed
  cursor interval, optional (table_id,record_id,revision). Reverse reference
  lookup for source fanout must be measured; use typed field indexes or a
  maintained relation only after contract approval, not an unbounded table
  scan hidden in a hook.
- Drafts: (owner_user_id,updated_at DESC,id DESC) plus unique id; composite
  app/table/view/target checks, schema/base/draftVersion constraints. Never
  store Session secrets or grant readable drafts to other owners.

If V030-013's hot7/cold4 land unoccupied, next candidate slots are hot8 for
record policy constraints, grant fields, draft/journal/revision and restricted
source mapping, and cold5 for minimal record audit. Recheck actual merged
HEAD and owner allocation before assigning filenames. Preserve hot1–7 and
cold1–4 bytes. Physical business rows remain typed; JSONB is acceptable only
for metadata, draft payload or restricted journal deltas. Draft payload is
not a committed record. The B5 operation table currently permits only 200/201
results and five operation kinds; a proposed draft-discard 204 requires a
reviewed additive operation kind/status constraint and an internal durable
result object, while the external 204 has no body. Record audit should
initially carry actor/app/table/
record/version/operation/changed field IDs and safe correlation, never raw
sensitive values. Future approval node old/new value trace needs a separate
redaction/view permission contract. Hot/cold role grants must be explicit and
least privilege; V030-015 does not edit shared roles or migration files now.

## 7. Independent RED/GREEN and scale acceptance after release

All items below are **NOT RUN**. Required evidence: source revision, isolated
PG18.6 and restricted auth_app role, exact independent expected rows/values,
initial target RED before code, committed GREEN, command/output/hash/time and
unrun/fail/skip distinction. Mock-only tests cannot certify these contracts.

| Matrix | Cases / oracle |
| --- | --- |
| Auth | two actors/two apps/two forms, owner/Bootstrap trusted flag spoof, create-only permission, no inherited child resource, exact own createdBy after editor changes, own-edit-X + other-read-Y cross-product, menu vs data, revoke/write races, field filter/sort/COUNT nonleakage |
| Typed records | every V030-013 field kind/default/null/decimal/time normalization, no JSONB business row, stable IDs, shared-table multi-view, schemaReady=false, field tombstone/unknown ID, required/option/ref rejection, CAS two writers, no-op |
| Transaction | row+audit+operation/optional draft rollback at every fault point, concurrent same operation key, committed response loss and original-key recovery, unknown PG commit error, no new-key retry |
| Fence/flow port | absent/unavailable fence fails closed; pending approval command blocks edit; future node Save field whitelist/CAS/change audit same tx and no automatic complete/other flow trigger |
| Query correctness | nested AND/OR, NULL eq/neq, text eq/neq only, decimal/date/datetime compare/order, duplicate/NULL refs, equal sort ID tie, page 1/deep/after-end, full authorized COUNT, hidden-column independence |
| Relevance | nonmatching row/field no refresh; entering/leaving filter, off-page visible edit, reorder, count change, source rename/delete/tombstone, policy grant/revoke, schema change, context eviction, concurrent snapshot/writer race |
| Draft | owner Session, cross-user/app/view denial, schema/base/draft CAS, explicit save no record/flow/revision side effect, same-tx consumption, later draft version survives failed/older submit, ambiguous commit/replay |
| Sources | real personnel many-department move, department rename, member delete, same-name new account, late source version, registry missing, references selected only while active, predicate before COUNT/LIMIT under same RR |
| Capacity | N=10k/100k/1m; selective/broad/own grants; 0/1/20 leaves and AND/OR; p=20/100 and o near 0/50k/500k; 1/20/200 active contexts; low/high edit rate; source fanout 1/1k/100k; A rehash and B E=0/100/10k/gap |
| Operations | EXPLAIN (ANALYZE,BUFFERS), count/page separately, warm/cold cache, index and temp bytes, WAL/write amplification, journal growth, DB/app RSS peak, p50/p95/p99 with repeated runs, lock wait/deadlocks, plan after ANALYZE |
| End to end | real HTTPS Session/CSRF, OpenAPI response validation, original Table queryVersion/refresh behavior, Chromium/Firefox/WebKit against actual PG API; no fixture-only completion claim |

No performance budget, query size cap, event retention or record deletion/
restoration strategy is asserted here. Benchmarks measure, then the lead
freezes limits and algorithm. A real B4a source-hook artifact and a real
pending-command fence remain integration blockers for those behaviors;
they do not block this review document.

## 8. Decisions needed from lead before code

1. Freeze/adjust routes, record/draft DTO, idempotent no-op, queryVersion write
   use, schemaReady error, error names and body limits. Record the narrow
   ADR-002 exceptions for record random-page and read-only POST search; do
   not extend the personnel-only Q25/PR21 exceptions by implication. Set
   whether form or table is the data grant resource and exact
   create-mask/empty-mask semantics.
2. Choose mixed row-field filter/sort rule (exclude unreadable rows or reject
   query) and complete projection boundary. Confirm policy/schema change
   behavior relative to QUERY_CHANGED and existing Q36 refresh semantics.
3. Choose Candidate A/B/hybrid after cost and correctness review, including
   journal retention and sensitive delta storage if B. Approve index budget
   and same-RR reference strategy; no fixed production latency claim.
4. Assign shared applications transaction lifecycle, apppolicy action extension,
   OpenAPI/errors, hot/cold migration numbers and roles to one owner. V030-015
   cannot write V030-013's shared files concurrently.
5. Provide authoritative multi-department source mapping/hooks and pending
   approval-command fence. Until then, dependent record operations fail
   closed; an empty adapter is not a test or delivery.
