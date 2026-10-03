# V030-013 product contract proposal

Review status: lead approved 2980b5e with the atomic-form-create and 12-column
span amendments below. HTTP wiring waits only for the matching Notion readback.
The V030-013 technical PLAN was read from PRD at
2026-10-03T08:40:01.401Z and ADR009 at 2026-10-03T08:40:07.467Z; the
two PLAN sections are byte-identical. This document supplies precise derived
contracts for the lead to freeze; it does not amend product decisions.

Base: PR26 actual HEAD `9b89e8e928aedf30df235492fe89bc52021f6fe9`.
B2 reusable source: `04c91479e24de95fc6b25ba9efc76a8f6b72de5f` (PR25).
All routes below are relative to `/api/v1/applications/{appId}` unless stated.

## 1. Common HTTP rules

- Ordinary JSON uses the ADR002 `code/message/data/meta` envelope and
  OpenAPI 3.2.1. `meta` has `requestId`; responses use `Cache-Control: no-store`.
- Canonical lowercase UUIDs identify apps, directories, logical tables, forms,
  fields, options, layout nodes and operations. New persisted directory/table/view
  IDs are server-generated. New fields/options/layout nodes accept client-generated
  UUIDs so an unsaved design has stable references; IDs cannot be reused from
  another app/table, and removed field IDs cannot be assigned a new meaning.
- Names use the existing B5 TrimSpace, 1–100 Unicode codepoints, no NUL rule;
  duplicate display names remain permitted. SQL never uses display names.
- Versions are JSON integers in the exact-safe range 0–9007199254740991.
  Policy revision retains B5's existing 1-based range. No overflow wraps.
- `structureVersion` is a proposed independent app metadata CAS for directory,
  table and form placement/name edits. It is not a policy, schema, view or record
  version. Fields/config and layout changes do not increment policy revision.
- Current trusted Session/active account/auth_version is rechecked. Writes and
  preflight require the existing same-origin/CSRF proof and strict JSON handling
  (1 MiB, no duplicate keys, no unknown keys, no missing required keys).
  GETs have no query parameters in this slice. Bodies are JSON objects.
- Configuration reads/writes currently require fixed owner or trusted Bootstrap.
  A normal member's application-root menu does not grant a child resource.
  Later ordinary-reader ports must resolve actual same-app resource registration
  and evaluate current menu/data grants; disconnected ports deny by default.
- Every persisted mutation includes client `operationId`. Actor+operationId is
  the shared B5 unique key, covering old and new methods/resources. Method,
  operation kind, route IDs and normalized complete business input (including
  supplied CAS versions and confirmation token) form its SHA-256 fingerprint.
  Replay is tested before current CAS/confirmation expiry, after current actor
  and result-readability checks. A committed replay returns the original result.
- PUTs below replace exactly the named resource document; omitted required keys
  are errors. Null parent/directory means root; empty arrays mean an intentional
  empty set. There is no implicit merge or JSON Merge Patch.
- Creating metadata reserves a logical table/form but runs **no business DDL**:
  initial schemaVersion=0, viewVersion=0, fields=[], layout=[], schemaReady=false.
  First definition Save creates the one shared physical table and makes the form
  usable. No separate form publish operation is introduced.
- No directory/table/form delete route is proposed: subtree deletion, restoration
  and retention semantics are not yet decided. Field removal remains part of Save
  with the approved impact/dependency guards. No new navigation persistence,
  flow action, notification or record CRUD route is added.

## 2. Routes and exact DTOs

`Write` below denotes `{operationId: UUID}`. All listed keys are required except
those explicitly marked optional. `position` is integer 0–2147483647; siblings
sort by `(position,id)`, so ties do not create nondeterministic order.

| Method and suffix | Request | Response data / status |
| --- | --- | --- |
| GET `/structure` | none | Structure / 200 |
| POST `/directories` | Write + `{name,parentId:UUID|null,position,expectedStructureVersion}` | DirectoryResult / 201 + Location |
| GET `/directories/{directoryId}` | none | DirectoryResult / 200 |
| PUT `/directories/{directoryId}` | Write + `{name,parentId:UUID|null,position,expectedStructureVersion}` | DirectoryResult / 200 |
| POST `/tables` | Write + `{name,directoryId:UUID|null,position,expectedStructureVersion}` | TableResult / 201 + Location |
| GET `/tables/{tableId}` | none | Table / 200 |
| PUT `/tables/{tableId}` | Write + `{name,directoryId:UUID|null,position,expectedStructureVersion}` | TableResult / 200 |
| POST `/forms` | Write + `{name,source:FormSource,directoryId:UUID|null,position,expectedStructureVersion}` | FormResult / 201 + Location |
| GET `/forms/{viewId}` | none | FormResult / 200 |
| PUT `/forms/{viewId}` | Write + `{name,directoryId:UUID|null,position,expectedStructureVersion}` | FormResult / 200 |
| GET `/forms/{viewId}/definition` | none | Definition / 200 |
| POST `/forms/{viewId}/definition/preflight` | DefinitionInput | Preflight / 200; no write operation is persisted |
| PUT `/forms/{viewId}/definition` | Write + DefinitionInput + `{confirmationToken:string|null}` | DefinitionSave / 200 |
| GET `/api/v1/application-operations/{operationId}` | none | Existing B5 Operation with additive structure result alternatives / 200 |

List/sibling collections come from the single coherent Structure response; this
proposal does not add duplicate collection list APIs. A view's tableId is
immutable. A second view is created with POST /forms pointing to the same table.
Tables and forms have independent directory placement. new_table creates the
same-name pending logical table and view atomically in the one operation/tx;
existing_table creates just the view. Failure leaves neither half. POST /tables
remains a separate explicit definition-management action, never a mandatory
preliminary frontend call before creating a form.

```ts
type Directory = { id:UUID; appId:UUID; name:string;
  parentId:UUID|null; position:number };
type Table = { id:UUID; appId:UUID; name:string; directoryId:UUID|null;
  position:number; schemaVersion:number; schemaReady:boolean };
type Form = { id:UUID; appId:UUID; tableId:UUID; name:string;
  directoryId:UUID|null; position:number; viewVersion:number };
type Structure = { appId:UUID; structureVersion:number;
  directories:Directory[]; tables:Table[]; forms:Form[];
  capabilities:{canManageDefinition:boolean} };
type DirectoryResult = { directory:Directory; structureVersion:number };
type TableResult = { table:Table; structureVersion:number };
type FormSource = {kind:"new_table"}|{kind:"existing_table";tableId:UUID};
type FormResult = { table:Table; form:Form; structureVersion:number };
type Definition = { appId:UUID; table:Table; form:Form;
  fields:Field[]; systemFields:SystemField[]; layout:LayoutNode[];
  capabilities:{canManageDefinition:boolean} };
type DefinitionInput = { expectedSchemaVersion:number;
  expectedViewVersion:number; fields:Field[]; layout:LayoutNode[];
  optionMappings:OptionMapping[] };
type DefinitionSave = { operationId:UUID; definition:Definition;
  appliedPlan:ChangePlan };
```

Both CAS values are checked even for a layout-only Save. Schema version increases
exactly once when normalized field metadata/config changes (including display
rename) or the initial physical table is created; a layout-only change does not
increase it. View version increases exactly once when that form's normalized
layout changes or its first Save occurs. A no-op Save persists one operation
result and minimal audit with unchanged versions; it does not invent a change.
Changing shared fields affects every view's definition, but does not increment
other views' layout versions. Save rejects a removed field still used in another
view's stored layout, and lists the views; it never rewrites another layout
without its owner's explicit versioned update. Flow dependencies remain separate.

## 3. Fields, defaults and presentation

Every Field has `{id:UUID,name:string,kind,required:boolean,default:Value|null,
config,presentation}`. `default:null` explicitly means no configured default;
it never becomes zero, false or empty text. A configured default is a constant
of the corresponding field kind; no SQL, expression, `now()` or actor macro is
accepted. Empty strings remain strings. All defaults are normalized/validated
through the same core as future record writes before DDL. SQL defaults and
constant old-row fill use exactly that normalized value.

`presentation` is `{helpText:string|null,displayTimeZone:string|null}`;
displayTimeZone is applicable only to datetime, uses a valid IANA timezone,
and defaults to UTC in normalized metadata. It never changes the stored instant.

| kind | config | Value on JSON wire | PG business column |
| --- | --- | --- | --- |
| text / multiline | `{maxLength:number|null}`; positive codepoint limit when supplied | string | text |
| number / money | DecimalConfig | decimal string | numeric(precision,scale) |
| date | `{}` | exact valid `YYYY-MM-DD` | date |
| datetime | `{precision:"minute"\|"second"\|"millisecond"}` | RFC3339 with explicit offset; normalized UTC | timestamptz |
| single_select | `{options:Option[]}` | option UUID | uuid |
| multi_select | `{options:Option[]}` | option UUID[] | uuid[] |
| boolean | `{}` | boolean | boolean |
| member / department | `{}` | stable source UUID | uuid |

`Option` is `{id:UUID,label:string}`; config order is presentation order and ID
survives a label rename. IDs are distinct per field. Multi-select values/defaults
are set-valued: deduplicate and normalize in option configuration order. This
does not authorize multi-member fields. New reference selections/defaults require
an active authoritative source adapter; absent adapters reject them. Historical
references have no cascade FK to live personnel; deleted sources keep their IDs
and last display through the approved source registry adapter.

```ts
type DecimalConfig = { precision:number; scale:number;
  roundingPlaces:number;
  roundingMode:"HALF_UP"|"HALF_EVEN"|"TOWARD_ZERO"|"FLOOR"|"CEILING" };
```

Proposal normalization: precision defaults to 38; money scale defaults to 2,
number scale to 0; roundingPlaces defaults to scale; roundingMode defaults to
HALF_UP. Full stored/response config always includes all four keys. Validate
1 <= precision <= 38, 0 <= scale <= min(18,precision), -18 <= roundingPlaces <= 18
and roundingPlaces <= scale. The last condition prevents a second unrequested
rounding by PostgreSQL's storage scale. Positive places are decimal digits after
the point; 0 means units; -1 tens; -2 hundreds. String grammar is
`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`; no exponent, NaN, Infinity or JSON float.
Compute once with exact integer arithmetic, then pad to storage scale and
normalize negative zero to positive zero. Check overflow **after** the configured
rounding. HALF_UP means half away from zero; HALF_EVEN selects an even retained
digit; FLOOR toward negative infinity; CEILING toward positive infinity.

Independent examples: HALF_UP(-1.25,1)=-1.3; HALF_EVEN(-1.25,1)=-1.2;
HALF_EVEN(1.35,1)=1.4; TOWARD_ZERO(-149,-2)=-100;
FLOOR(-149,-2)=-200; CEILING(-149,-2)=-100; HALF_UP(150,-2)=200.
numeric(4,2) with 99.995 rounded to cents overflows to 100.00 and is rejected.

Proposed datetime normalization floors the instant in UTC to the requested
minute/second/millisecond boundary (including instants before 1970); it does not
round across a future boundary. Default precision is second. A changed precision
is a value transformation guarded like a type/config change, not just display.
The exact behavior/default is for lead review; the PLAN does not specify it.

SystemField codes are fixed `id`, `createdBy`, `createdAt`, `updatedAt`,
`recordVersion`; response `{id:code,kind,readOnly:true}`. They are excluded from
the mutable fields array. System physical columns are `id uuid` PK,
`created_by uuid`, `created_at timestamptz`, `updated_at timestamptz`,
`record_version bigint`; creation and updates are filled by trusted record ports.
Client fields cannot alias or rename them. Physical user columns are
`f_<UUID without hyphens>`, tables `appdata.t_<UUID without hyphens>`.

`LayoutNode` is a tagged union with stable `id`:

- `{id,kind:"field",fieldId:UUID,span?:number}`
- `{id,kind:"system_field",fieldId:SystemFieldCode,span?:number}`
- `{id,kind:"group",title:string,children:LayoutNode[],span?:number}`
- `{id,kind:"divider"}`
- `{id,kind:"description",text:string}` (plain text)

Each group uses a 12-column grid. field/system_field/group span is integer
1–12, omitted span normalizes to 12. description/divider are always full row
and reject span. Placement/width changes increment only viewVersion.
Arrays define layout order. Each field appears at most once per view; referenced
fields exist in that table. Missing editable fields are allowed, so views can
show a subset without changing table data. Cycles/duplicate layout IDs fail
validation. Layout nodes create no business SQL columns. Fine visual dimensions
remain frontend presentation; unknown persisted config keys are rejected.

## 4. Preflight, destructive mapping and Save

```ts
type OptionMapping = { fieldId:UUID; fromOptionId:UUID;
  toOptionId:UUID|null };
type ChangePlan = { schemaChanges:SchemaChange[];
  metadataChanged:boolean; layoutChanged:boolean };
type SchemaChange = { kind:"add"|"remove"|"change_type"|"change_config"|
    "change_default"|"change_required";
  fieldId:UUID; beforeKind:FieldKind|null; afterKind:FieldKind|null };
type Impact = { fieldId:UUID;
  kind:"column_removal"|"option_mapping"; nonNullRows:number;
  optionId:UUID|null };
type Dependency = { fieldId:UUID;
  kind:"enabled_flow"|"in_flight"|"view_layout"; resourceId:UUID };
type Preflight = { appId:UUID; tableId:UUID; viewId:UUID;
  schemaVersion:number; viewVersion:number; dataRevision:number;
  dependencyRevision:number; plan:ChangePlan; impacts:Impact[];
  dependencies:Dependency[];
  blockingIssues:{code:string;fieldIds:UUID[]}[]; saveAllowed:boolean;
  confirmation:{token:string; expiresAt:string}|null };
```

Preflight uses one read-only RR snapshot for table/view metadata, dependency
revision, data revision and impact scans. It produces no DDL, metadata, audit or
operation ledger writes. Domain validation with an inspectable plan returns 200
with saveAllowed=false and dependencies/impacts; malformed/cross-app/CAS requests
use errors below. No blocked dependency can be overridden by confirmation.
blockingIssues uses the registered schema error codes below to explain required
backfill, unsupported/invalid conversion, option mapping or cross-view removal
issues without returning raw business values. Collections always return arrays.

Used option deletion needs a complete explicit from->to mapping: target option
exists in the new config, or null explicitly clears to NULL/omits that element
for multi-select. Clearing a required value rejects Save. Resulting multi-values
are deduplicated; empty optional selection normalizes to NULL. A used option
without mapping blocks Save. Mapped destructive changes are included in impact
confirmation. Multi->single rejects any old row with more than one distinct
value **before** mappings; a mapping cannot disguise or bypass that rule.

Confirmation proposal: opaque versioned HMAC-SHA256 token with a dedicated
server key and key ID; 10-minute expiry as an operational policy, returned as
expiresAt so callers do not hard-code a duration. It is bound to actor/app/table,
view, expected schema/view versions, data revision, dependency revision and
normalized plan hash (fields, layout, option mappings, semantic config). HMAC key
is injected; no checked-in/default shared key or secret in logs. Other-view layout
references participate in dependency revision. Expired/tampered/wrong-scope/stale
tokens fail closed. Tokens are verified only after table/data/dependency locks
are held. No record values or user secrets are embedded in tokens.

New required fields on old rows require a normalized constant default in this
slice; otherwise Save returns REQUIRED_BACKFILL. Arbitrary per-record repair is
outside this API: old data must be filled through future authorized record ports
before changing an existing column to required. No implicit null-to-zero fill.
Physical conversion uses an explicit whitelist: exact parse into the target
type; same-family decimal/time precision uses this normalization core; single->
multi promotes one option ID; multi->single applies the strict cardinality guard;
scalar->text uses canonical serialization. Cross-source member<->department and
unsupported conversions reject; they never reinterpret UUIDs silently. Parse,
required, option or overflow failure rolls back the complete Save. Error details
contain field IDs/reasons/counts only, no old sensitive values or raw PG errors.

## 5. Errors

Existing B5/session/media errors retain their HTTP meanings. Shared codes below
are proposed additive registrations; no production errors file is changed yet.

| Code | HTTP | data |
| --- | --- | --- |
| COMMON_VALIDATION_FAILED | 400 | existing ValidationErrorData using body field paths |
| APPLICATION_FORBIDDEN | 403 | null; ordinary-member disconnected paths deny |
| APPLICATION_NOT_FOUND | 404 | null; also wrong app ownership for route resource |
| APPLICATION_RESOURCE_INVALID | 400 | null; unknown/cross-app input references |
| APPLICATION_OPERATION_CONFLICT | 409 | null; shared actor/key collision |
| APPLICATION_STRUCTURE_CONFLICT | 409 | `{currentStructureVersion}` |
| APPLICATION_SCHEMA_CONFLICT | 409 | `{currentSchemaVersion}` |
| APPLICATION_VIEW_CONFLICT | 409 | `{currentViewVersion}` |
| APPLICATION_SCHEMA_DEPENDENCY_BLOCKED | 409 | `{dependencies:Dependency[]}` |
| APPLICATION_SCHEMA_REQUIRED_BACKFILL | 409 | `{fieldIds:UUID[]}` |
| APPLICATION_SCHEMA_CONVERSION_FAILED | 409 | `{fieldIds:UUID[]}` |
| APPLICATION_SCHEMA_OPTION_MAPPING_REQUIRED | 409 | `{fieldIds:UUID[]}` |
| APPLICATION_SCHEMA_CONFIRMATION_REQUIRED | 409 | `{impacts:Impact[]}` |
| APPLICATION_SCHEMA_CONFIRMATION_STALE | 409 | `{reason:"expired"|"context_changed"}` |
| APPLICATION_OPERATION_UNCONFIRMED | 503 | `{operationId:UUID}`; query original key |
| COMMON_SERVICE_UNAVAILABLE | 503 | null; dependency adapter absent/unavailable or lock/statement timeout before commit |

Invalid signature or wrong actor scope uses COMMON_VALIDATION_FAILED rather than
revealing another actor's token context. Only current authorized readers see
current version/impact/dependency details. Dependency availability is never
represented by an empty list on failure. Only confirmed COMMIT sends success.
A COMMIT error without explicit rollback proof remains UNCONFIRMED, including
server SQLSTATE errors. GET operation 404 never proves rollback; retry keeps the
original key and exact body. Post-commit session renewal failure retains confirmed
success while clearing the invalid session, following B5.

## 6. Caller-owned transaction ports and lock order

The existing B2 Save opens/commits its own transaction. Preserve that compatibility
entry but factor its body into `ApplyInTx` which never Begin/Commit/Rollback:

```go
func (e appschema.Executor) ApplyInTx(ctx context.Context, tx pgx.Tx,
    request appschema.Request) (appschema.Result, error)
// Existing Save opens one tx, calls ApplyInTx, classifies COMMIT once.
```

Proposed B5 shared lifecycle adapter (`internal/applications/transactions.go`):

```go
func (a *Application) BeginManagerWrite(context.Context, session.Principal,
    appID string) (*ManagerWrite, error)
func (w *ManagerWrite) Tx() pgx.Tx
func (w *ManagerWrite) Replay(context.Context, operationID, kind string,
    fingerprint [32]byte) (*Result, error)
func (w *ManagerWrite) Claim(context.Context, operationID, kind string,
    fingerprint [32]byte) error
func (w *ManagerWrite) Complete(context.Context, operationID string, Result) error
func (w *ManagerWrite) Commit(context.Context) error // B5 unknown classification
func (w *ManagerWrite) Rollback(context.Context) error
```

New appstructure adapters own Metadata/Guard/Dependencies/Confirmation and
minimal structural audit in that same tx. New appfields normalization is pure;
reference activity is a mandatory caller adapter when a reference value is new.
The future record/query/Flowable ports take caller pgx.Tx and trusted resource
context; they cannot start independent writes or replace missing guards with true.

Lock order extends, without reversing, PR21/B5:

1. Set explicit transaction-local lock/statement limits, then acquire all PR21
   `personnel.lock_query_revisions()` locks in its existing fixed order.
2. `AccessForWrite` locks the current account, member_configuration, identities,
   templates and permission-catalog dependencies in its existing order; recheck
   status/auth_version. Source validation locks sorted UUID source dependencies
   at this layer, never after the app/data locks.
3. App row/policy gate, then operation claim (actor+operationId uniqueness).
4. App structure gate, table metadata gates sorted table UUID; same-table gate
   serializes schema changes, data-revision writers and flow/view dependency
   registrations. Lock affected view rows in sorted UUID order, including other
   stored view references relevant to removal.
5. Physical table ACCESS EXCLUSIVE for any value/structure change. Read current
   data/dependency revisions and inspect protected values; verify confirmation
   under these locks. Record writers/dependency registration must take the same
   table gate before touching rows/flow references, never the reverse order.
6. Controlled DDL/data conversion; fields/layout/version metadata; resource
   registration after its owning metadata exists; minimal audit; operation result.
7. One COMMIT owned by ManagerWrite. No remote Flowable call inside these locks.

Read-only definition/structure/preflight uses one short RR transaction for live
actor/resource/policy and all metadata/impact facts; no read-side cache writes.
App gate currently serializes same-app configuration; DDL blocks same-table
reads/writes until commit. This is synchronous Save, not a low-downtime claim.

Protected DDL proposal: `auth_app` gets no CREATE on appdata, no physical table
ownership and no generic EXECUTE-SQL function. Migration-owned, schema-qualified
SECURITY DEFINER routines accept canonical table/field IDs, allowlisted operations
and typed config/constants, construct identifiers internally and validate the
same-app registry. PUBLIC execution is revoked; search_path=pg_catalog. The B2
executor routes typed DDL through that port while retaining direct SQL only in
its isolated legacy test adapter. Runtime privileges, no arbitrary ALTER/CREATE,
crafted identifiers/constants and transactional rollback need real restricted-
role tests before the interface is considered delivered.

## 7. Exact scope and migration allocation

V030-013 exclusively owns new `services/bff/internal/appfields/`,
`services/bff/internal/appstructure/`, reused/extended
`services/bff/internal/appschema/`, `contracts/application-structure*`,
`docs/tasks/V030-013.md`, `docs/evidence/V030-013/`.

The following shared-file registration is proposed for lead acknowledgement
**before implementation**, distinct from ownership of PR26 CI repairs:

- `services/bff/internal/applications/transactions.go` and directly related tests;
  `types.go`/`store.go`/`http.go` only for typed lifecycle, resource registration,
  definition-handler delegation and operation result compatibility.
- `services/bff/cmd/bff/config.go` and `definition_wiring_test.go` (actual composition
  is config.go, not main.go); delegate specific definition routes from existing
  applications service, leaving auth/Session routing unchanged.
- `contracts/openapi/openapi.json`, `contracts/errors/codes.json` and new contract
  tests; no frontend-generated copies or direct edits by V030-012.
- New hot `db/migrations/00007_app_structure.sql` and cold
  `db/archive-migrations/00004_app_structure_audit.sql`. PR26 uses hot1–6,
  cold1–3. Preserve every earlier migration byte. Proposed hot metadata:
  directories, logical_tables, fields (stable-ID tombstones), form_views,
  table_field_dependencies, table data/dependency revisions, app structure
  revision, scoped resource registration, new operation kinds and controlled DDL.
  Metadata/config/layout may use JSONB; business rows have physical typed columns.
  Dependency registration is local authoritative storage, not a guessed Flowable
  status; absent availability adapters block destructive Save.
- `infra/runtime/roles.sql`/`cold-roles.sql` append explicit new capabilities,
  preserve old statements. No runtime owner transfer or broad schema CREATE.
  New structural audit needs hot+cold compatible allowlisted summaries; existing
  B5 summary restrictions remain intact. No data/default/layout values in audit.

Extra CI reuse requirement discovered from the fixed B2 source: its storage tests
require the dedicated `WEAVEOS_B2_TEST_DATABASE_URL` private socket fixture,
which PR26 does not contain. To retain its original regression tests, separately
request only the PR25 B2 lifecycle sections in `.github/workflows/ci.yml`,
`infra/acceptance/run.mjs` and `tests/foundation/b2-ci-database.test.mjs`
(the exact path was verified from fixed PR25). This is not
authority to change the ongoing PR26 CI repairs or original acceptance gates.
No go.mod/go.sum/pnpm edits or dependency additions are proposed.

## 8. Required RED/GREEN matrix

| Independent source | Target checks |
| --- | --- |
| AP-FR01/02 + PLAN1 | true persisted directory/table/form, cycle and cross-app rejection, same table two views, immutable IDs, pending->first Save; definition read proves resource owner/Bootstrap/current Session; root-menu/member/create-only cannot read child data |
| PLAN2 decimal | five modes, negative halves, tens/hundreds and -18/+18, exact large strings, default normalization, precision/scale invalid combinations, NaN/exponent/float rejection, post-round overflow |
| PLAN2 time | leap/invalid dates, mandatory offset, UTC instant, minute/second/millisecond including pre-epoch boundaries and offsets; display timezone changes do not alter stored values |
| AP-FR02 + PLAN2/3 options | stable option rename, default/multi dedup, unknown IDs, mapping required, required-clear rejection, multi->single cardinality failure rolls back all fields/layout |
| AP-FR02/08 + PLAN3 | NULL/default old rows, new required old-row guard, existing nullable->required, cast invalid old values, enabled/in-flight references and missing protector block; other-view layout references protected |
| PLAN3 CAS/confirmation | two-session schema/view races; layout-only CAS; data change with same row count and dependency-only change invalidate token; wrong actor/app/view/plan, expired/tampered token; replay after expiry succeeds only for already committed exact key |
| PLAN3 atomicity | injected failure at DDL/metadata/layout/audit/result, confirm all-or-none via independent catalog/type/value/revision/audit queries; one caller transaction, no internal commit |
| B5 + PLAN3 | actual committed change with lost commit response, original operation query and exact-key replay, cross-kind/app key conflict, no duplicate audit/revision; ambiguous PG error stays unconfirmed |
| PLAN4/5 roles | real auth_app cannot arbitrary CREATE/ALTER/drop or call definer via PUBLIC; controlled Save succeeds; literal injection cannot escape; cold/hot append audit and restricted backup/restore; empty DB and hot6/cold3 upgrade preserve original migrations and owner |
| Existing contracts | OpenAPI3.2.1 schemas, statuses, Location, strict complete DTOs/null/empty semantics, Session/Origin/CSRF, B5/personnel/query/backup regressions, full same-head CI |

Preparation TDD:N/A (documentation only). All implementation RED/GREEN above is
**NOT RUN** at this checkpoint. Source/code inspection and old package evidence
are not new tests or product acceptance.

## 9. Lead review items

The lead froze the following choices and approved §7 shared scope, with the
atomic creation and span amendments. Read back matching Notion before HTTP wiring:
pending table/form creation with first-Save DDL; independent structureVersion CAS;
roundingPlaces sign/defaults and storage-scale compatibility; UTC time truncation;
other-view field removal guard; constant default-only old-row fill and explicit
option mappings; confirmation expiry/key configuration; restricted typed-DDL port.
The PLAN releases field core/schema tests and implementation independently; this
proposal does not postpone that authorized work pending all HTTP decisions.
