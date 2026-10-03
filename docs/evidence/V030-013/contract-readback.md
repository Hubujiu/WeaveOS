# Accepted task-specific ADR readback
Source: https://app.notion.com/p/3ee2f5a9e64881489017e2f38a4bb29f
page_last_edited_at: 2026-10-03T09:09:21.333Z
Status: accepted finite task contract; no full implementation/product acceptance.

Here is the result of "fetch" for the Page with URL https://app.notion.com/p/3ee2f5a9e64881489017e2f38a4bb29f as of 2026-10-03T09:09:21.675Z:
<page url="https://app.notion.com/p/3ee2f5a9e64881489017e2f38a4bb29f" icon="📐">
<ancestor-path>
<parent-data-source url="collection://235f72c0-b039-40d8-9dcc-7abff28b9406" name="架构决策 ADR"/>
<ancestor-2-database url="https://app.notion.com/p/f4b3520a9eb84c8280f33f35142ca388" title="架构决策 ADR"/>
<ancestor-3-page url="https://app.notion.com/p/3e52f5a9e64881b9b005dc78a252c08f" title="Architecture & ADR"/>
<ancestor-4-page url="https://app.notion.com/p/3e42f5a9e64880ae9cf5ebbd2088d773" title="项目 - 企业管理系统"/>
</ancestor-path>
<properties>
{"ADR 标题":"V030-013 ADR｜应用结构与表单定义精确执行合同","ADR 编号":"","date:决策日期:is_datetime":0,"date:决策日期:start":"2026-10-03","url":"https://app.notion.com/p/3ee2f5a9e64881489017e2f38a4bb29f","关联迭代":["https://app.notion.com/p/3ed2f5a9e648811abf4ee555fb9695aa"],"决策领域":"API","影响范围":"模块级","最后更新时间":"2026-10-03T09:09:21.333Z","状态":"已接受"}
</properties>
<iconMetadata>{"type":"emoji","emoji":"📐"}</iconMetadata>
<content>
本页是 **V030-013 的权威执行合同**，主负责人已冻结本任务技术选择，尚无本任务完整实现／验收通过声明。总 PRD／ADR 的历史表格曾因 nullable 联合类型拆列而未完整呈现；字段、方法、错误与端口以本独立页的可读清单及代码定义为准，不依赖旧表格。
上位需求：<mention-page url="https://app.notion.com/p/3ed2f5a9e648811abf4ee555fb9695aa"/>。上位架构：<mention-page url="https://app.notion.com/p/3ed2f5a9e64881258f3afbd0f83bca3f"/>。任务 PRD 创建后互链；整体 v0.3.0 状态不改变。
来源：[固定 2980b5e 的源码 proposal](https://github.com/Hubujiu/WeaveOS/blob/2980b5e89dd4e86af5b2a720346d8871384a4911/contracts/application-structure.proposal.md)。主负责人已读全合同，本页包含后续两项更正与输入／输出默认值澄清；不声称该固定 proposal 已被修改。
## 冻结结论与优先更正
1. 新建表单 POST forms 采用 source 判别对象：new_table 同事务创建逻辑表和 form，继承 form 的名称与位置，初始 schema／view 版本为 0，首次 Save 才执行 DDL；existing_table 引用已有同 app 逻辑表。FormResult 含 table、form、structureVersion。独立 POST tables 保留，新表单 UI 不拆两次创建。
2. field／system_field／group 的 span 为 1–12，省略规范成 12；group 内 12 列网格，description／divider 满宽，数组排序。纯布局不改业务列或值。
3. DecimalConfig 输入四键 optional、config 对象 required；仅省略采用默认，null 数值和未知键拒绝。输出／存储四键完整。缺省 roundingPlaces 取规范化 scale；幂等指纹使用完整规范化 DTO，省略默认与显式相同值等价。
4. datetime precision 输入可省略，默认 second；displayTimeZone 输入可空／省略，仅 datetime 规范为 UTC。不得把所有字段键都变 optional。
5. schema／view／structure CAS、默认值、映射、decimal、UTC floor、跨 view 删除保护、HMAC 专用注入 key／keyId／10 分钟 expiresAt 均由主负责人冻结。不得生成或配置真实生产凭据。
6. 真实本地依赖 registry 在无流程且可用时可以为空；不可用不能伪造空值。owner／Bootstrap 管理定义，普通业务资源权限留后续接线。
以下精确方法、DTO、错误码、端口与 §7 文件归属由主负责人批准。此合同没有增加删除／恢复、普通记录 CRUD、流程动作或导航持久策略，用户待答的四项问题保持独立。
## 合同 1｜Common HTTP rules
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
- `structureVersion` is an independent app metadata CAS for directory,
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
	initial schemaVersion=0, viewVersion=0, fields=\[\], layout=\[\], schemaReady=false.
	First definition Save creates the one shared physical table and makes the form
	usable. No separate form publish operation is introduced.
- No directory/table/form delete route is included: subtree deletion, restoration
	and retention semantics are not yet decided. Field removal remains part of Save
	with the approved impact/dependency guards. No new navigation persistence,
	flow action, notification or record CRUD route is added.
## 合同 2｜Routes and exact DTOs
`Write` below denotes `{operationId: UUID}`. All listed keys are required except
those explicitly marked optional. `position` is integer 0–2147483647; siblings
sort by `(position,id)`, so ties do not create nondeterministic order.
- **Method and suffix**：GET `/structure`；**Request**：none；**Response data / status**：Structure / 200
- **Method and suffix**：POST `/directories`；**Request**：Write + `{name,parentId:UUID|null,position,expectedStructureVersion}`；**Response data / status**：DirectoryResult / 201 + Location
- **Method and suffix**：GET `/directories/{directoryId}`；**Request**：none；**Response data / status**：DirectoryResult / 200
- **Method and suffix**：PUT `/directories/{directoryId}`；**Request**：Write + `{name,parentId:UUID|null,position,expectedStructureVersion}`；**Response data / status**：DirectoryResult / 200
- **Method and suffix**：POST `/tables`；**Request**：Write + `{name,directoryId:UUID|null,position,expectedStructureVersion}`；**Response data / status**：TableResult / 201 + Location
- **Method and suffix**：GET `/tables/{tableId}`；**Request**：none；**Response data / status**：Table / 200
- **Method and suffix**：PUT `/tables/{tableId}`；**Request**：Write + `{name,directoryId:UUID|null,position,expectedStructureVersion}`；**Response data / status**：TableResult / 200
- **Method and suffix**：POST `/forms`；**Request**：Write + `{name,source:FormSource,directoryId:UUID|null,position,expectedStructureVersion}`；**Response data / status**：FormResult / 201 + Location
- **Method and suffix**：GET `/forms/{viewId}`；**Request**：none；**Response data / status**：FormResult / 200
- **Method and suffix**：PUT `/forms/{viewId}`；**Request**：Write + `{name,directoryId:UUID|null,position,expectedStructureVersion}`；**Response data / status**：FormResult / 200
- **Method and suffix**：GET `/forms/{viewId}/definition`；**Request**：none；**Response data / status**：Definition / 200
- **Method and suffix**：POST `/forms/{viewId}/definition/preflight`；**Request**：DefinitionInput；**Response data / status**：Preflight / 200; no write operation is persisted
- **Method and suffix**：PUT `/forms/{viewId}/definition`；**Request**：Write + DefinitionInput + `{confirmationToken:string|null}`；**Response data / status**：DefinitionSave / 200
- **Method and suffix**：GET `/api/v1/application-operations/{operationId}`；**Request**：none；**Response data / status**：Existing B5 Operation with additive structure result alternatives / 200
List/sibling collections come from the single coherent Structure response; this
contract does not add duplicate collection list APIs. A view's tableId is
immutable. POST /forms with source=\{kind:"new_table"\} atomically creates a new
logical table and form in one transaction; the new table inherits the form name
and directory/position, with initial schemaVersion=0 and viewVersion=0 and no
business DDL until definition Save. source=\{kind:"existing_table",tableId\}
creates a view of an existing same-app table. FormResult always includes table,
form and structureVersion. Independent POST /tables is retained, but the new-form
UI must not implement new_table as two partial creation requests. Tables and
forms may subsequently have independent directory placement.
```typescript
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
type FormSource = {kind:"new_table"} | {kind:"existing_table";tableId:UUID};
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
## 合同 3｜Fields, defaults and presentation
Every Field has \`\{id:UUID,name:string,kind,required:boolean,default:Value\|null,
config,presentation\}`. `default:null\` explicitly means no configured default;
it never becomes zero, false or empty text. A configured default is a constant
of the corresponding field kind; no SQL, expression, `now()` or actor macro is
accepted. Empty strings remain strings. All defaults are normalized/validated
through the same core as future record writes before DDL. SQL defaults and
constant old-row fill use exactly that normalized value.
Input presentation keeps required helpText:string\|null and permits
optional displayTimeZone?:string\|null. For datetime, an omitted/null
	displayTimeZone normalizes to UTC; a supplied value must be a valid IANA timezone.
The setting is applicable only to datetime and never changes the stored instant;
other field kinds do not receive an automatic UTC setting. Stored/response
presentation uses \{helpText:string\|null,displayTimeZone:string\|null\}.
This exception does not make every field/config/presentation key optional.
- **kind**：text / multiline；**config**：`{maxLength:number|null}`; positive codepoint limit when supplied；**Value on JSON wire**：string；**PG business column**：text
- **kind**：number / money；**config**：input DecimalConfigInput; stored/response DecimalConfig；**Value on JSON wire**：decimal string；**PG business column**：numeric(precision,scale)
- **kind**：date；**config**：`{}`；**Value on JSON wire**：exact valid `YYYY-MM-DD`；**PG business column**：date
- **kind**：datetime；**config**：input `{precision?:"minute"|"second"|"millisecond"}`; normalized precision always present；**Value on JSON wire**：RFC3339 with explicit offset; normalized UTC；**PG business column**：timestamptz
- **kind**：single_select；**config**：`{options:Option[]}`；**Value on JSON wire**：option UUID；**PG business column**：uuid
- **kind**：multi_select；**config**：`{options:Option[]}`；**Value on JSON wire**：option UUID\[\]；**PG business column**：uuid\[\]
- **kind**：boolean；**config**：`{}`；**Value on JSON wire**：boolean；**PG business column**：boolean
- **kind**：member / department；**config**：`{}`；**Value on JSON wire**：stable source UUID；**PG business column**：uuid
`Option` is `{id:UUID,label:string}`; config order is presentation order and ID
survives a label rename. IDs are distinct per field. Multi-select values/defaults
are set-valued: deduplicate and normalize in option configuration order. This
does not authorize multi-member fields. New reference selections/defaults require
an active authoritative source adapter; absent adapters reject them. Historical
references have no cascade FK to live personnel; deleted sources keep their IDs
and last display through the approved source registry adapter.
```typescript
type DecimalConfigInput = { precision?:number; scale?:number;
  roundingPlaces?:number;
  roundingMode?:"HALF_UP"|"HALF_EVEN"|"TOWARD_ZERO"|"FLOOR"|"CEILING" };
type DecimalConfig = { precision:number; scale:number;
  roundingPlaces:number;
  roundingMode:"HALF_UP"|"HALF_EVEN"|"TOWARD_ZERO"|"FLOOR"|"CEILING" };
```
Frozen input/output distinction: config itself is required. DecimalConfigInput's
four keys are optional; unknown keys and null numeric/config values are rejected.
Omission selects a default and is not equivalent to null. The operation
fingerprint uses the fully normalized DTO, so omitted defaults and explicitly
supplied equivalent values are the same business payload.
Normalization: precision defaults to 38; money scale defaults to 2,
number scale to 0; roundingPlaces defaults to scale; roundingMode defaults to
HALF_UP. Full stored/response config always includes all four keys. Validate
1 \<= precision \<= 38, 0 \<= scale \<= min(18,precision), -18 \<= roundingPlaces \<= 18
and roundingPlaces \<= scale. The last condition prevents a second unrequested
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
Frozen datetime normalization floors the instant in UTC to the requested
minute/second/millisecond boundary (including instants before 1970); it does not
round across a future boundary. Default precision is second. A changed precision
is a value transformation guarded like a type/config change, not just display.
Input datetime config.precision is optional and defaults to second; unknown
keys or null precision are rejected. Stored/response config contains the normalized
precision. This behavior/default is now frozen by the lead.
SystemField codes are fixed `id`, `createdBy`, `createdAt`, `updatedAt`,
`recordVersion`; response `{id:code,kind,readOnly:true}`. They are excluded from
the mutable fields array. System physical columns are `id uuid` PK,
`created_by uuid`, `created_at timestamptz`, `updated_at timestamptz`,
`record_version bigint`; creation and updates are filled by trusted record ports.
Client fields cannot alias or rename them. Physical user columns are
`f_<UUID without hyphens>`, tables `appdata.t_<UUID without hyphens>`.
`LayoutNode` is a tagged union with stable `id`:
- input `{id,kind:"field",fieldId:UUID,span?:number}`
- input `{id,kind:"system_field",fieldId:SystemFieldCode,span?:number}`
- input `{id,kind:"group",title:string,children:LayoutNode[],span?:number}`
- `{id,kind:"divider"}`
- `{id,kind:"description",text:string}` (plain text)
For field/system_field/group, span is an integer 1–12; omission normalizes to 12
and stored/response nodes include the normalized value. Each group has its own
12-column grid. Description/divider use full width. Arrays define layout order;
a pure layout change does not alter business columns or record values.
Each field appears at most once per view; referenced
fields exist in that table. Missing editable fields are allowed, so views can
show a subset without changing table data. Cycles/duplicate layout IDs fail
validation. Layout nodes create no business SQL columns. Fine visual dimensions
remain frontend presentation; unknown persisted config keys are rejected.
## 合同 4｜Preflight, destructive mapping and Save
```typescript
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
Used option deletion needs a complete explicit from-\>to mapping: target option
exists in the new config, or null explicitly clears to NULL/omits that element
for multi-select. Clearing a required value rejects Save. Resulting multi-values
are deduplicated; empty optional selection normalizes to NULL. A used option
without mapping blocks Save. Mapped destructive changes are included in impact
confirmation. Multi-\>single rejects any old row with more than one distinct
value **before** mappings; a mapping cannot disguise or bypass that rule.
Frozen confirmation contract: opaque versioned HMAC-SHA256 token with a dedicated
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
type; same-family decimal/time precision uses this normalization core; single-\>
multi promotes one option ID; multi-\>single applies the strict cardinality guard;
scalar-\>text uses canonical serialization. Cross-source member\<-\>department and
unsupported conversions reject; they never reinterpret UUIDs silently. Parse,
required, option or overflow failure rolls back the complete Save. Error details
contain field IDs/reasons/counts only, no old sensitive values or raw PG errors.
## 合同 5｜Errors
Existing B5/session/media errors retain their HTTP meanings. Shared codes below
are approved additive registrations for implementation; this document does not
claim the production errors file or a deployed environment has been changed.
- **Code**：COMMON_VALIDATION_FAILED；**HTTP**：400；**data**：existing ValidationErrorData using body field paths
- **Code**：APPLICATION_FORBIDDEN；**HTTP**：403；**data**：null; ordinary-member disconnected paths deny
- **Code**：APPLICATION_NOT_FOUND；**HTTP**：404；**data**：null; also wrong app ownership for route resource
- **Code**：APPLICATION_RESOURCE_INVALID；**HTTP**：400；**data**：null; unknown/cross-app input references
- **Code**：APPLICATION_OPERATION_CONFLICT；**HTTP**：409；**data**：null; shared actor/key collision
- **Code**：APPLICATION_STRUCTURE_CONFLICT；**HTTP**：409；**data**：`{currentStructureVersion}`
- **Code**：APPLICATION_SCHEMA_CONFLICT；**HTTP**：409；**data**：`{currentSchemaVersion}`
- **Code**：APPLICATION_VIEW_CONFLICT；**HTTP**：409；**data**：`{currentViewVersion}`
- **Code**：APPLICATION_SCHEMA_DEPENDENCY_BLOCKED；**HTTP**：409；**data**：`{dependencies:Dependency[]}`
- **Code**：APPLICATION_SCHEMA_REQUIRED_BACKFILL；**HTTP**：409；**data**：`{fieldIds:UUID[]}`
- **Code**：APPLICATION_SCHEMA_CONVERSION_FAILED；**HTTP**：409；**data**：`{fieldIds:UUID[]}`
- **Code**：APPLICATION_SCHEMA_OPTION_MAPPING_REQUIRED；**HTTP**：409；**data**：`{fieldIds:UUID[]}`
- **Code**：APPLICATION_SCHEMA_CONFIRMATION_REQUIRED；**HTTP**：409；**data**：`{impacts:Impact[]}`
- **Code**：APPLICATION_SCHEMA_CONFIRMATION_STALE；**HTTP**：409；**data**：`{reason:"expired"|"context_changed"}`
- **Code**：APPLICATION_OPERATION_UNCONFIRMED；**HTTP**：503；**data**：`{operationId:UUID}`; query original key
- **Code**：COMMON_SERVICE_UNAVAILABLE；**HTTP**：503；**data**：null; dependency adapter absent/unavailable or lock/statement timeout before commit
Invalid signature or wrong actor scope uses COMMON_VALIDATION_FAILED rather than
revealing another actor's token context. Only current authorized readers see
current version/impact/dependency details. Dependency availability is never
represented by an empty list on failure. Only confirmed COMMIT sends success.
A COMMIT error without explicit rollback proof remains UNCONFIRMED, including
server SQLSTATE errors. GET operation 404 never proves rollback; retry keeps the
original key and exact body. Post-commit session renewal failure retains confirmed
success while clearing the invalid session, following B5.
## 合同 6｜Caller-owned transaction ports and lock order
The existing B2 Save opens/commits its own transaction. Preserve that compatibility
entry but factor its body into `ApplyInTx` which never Begin/Commit/Rollback:
```go
func (e appschema.Executor) ApplyInTx(ctx context.Context, tx pgx.Tx,
    request appschema.Request) (appschema.Result, error)
// Existing Save opens one tx, calls ApplyInTx, classifies COMMIT once.
```
Frozen B5 shared lifecycle adapter (`internal/applications/transactions.go`):
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
Frozen protected DDL contract: `auth_app` gets no CREATE on appdata, no physical table
ownership and no generic EXECUTE-SQL function. Migration-owned, schema-qualified
SECURITY DEFINER routines accept canonical table/field IDs, allowlisted operations
and typed config/constants, construct identifiers internally and validate the
same-app registry. PUBLIC execution is revoked; search_path=pg_catalog. The B2
executor routes typed DDL through that port while retaining direct SQL only in
its isolated legacy test adapter. Runtime privileges, no arbitrary ALTER/CREATE,
crafted identifiers/constants and transactional rollback need real restricted-
role tests before the interface is considered delivered.
## 合同 7｜Exact scope and migration allocation
V030-013 exclusively owns new `services/bff/internal/appfields/`,
`services/bff/internal/appstructure/`, reused/extended
`services/bff/internal/appschema/`, `contracts/application-structure*`,
`docs/tasks/V030-013.md`, `docs/evidence/V030-013/`.
The following exact shared-file registration is approved for V030-013 by the lead,
distinct from ownership of PR26 CI repairs; coordinate single ownership before
editing so ongoing B5 runtime repairs are not modified concurrently:
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
	cold1–3. Verify these numbers remain unoccupied against the actual implementation HEAD
	before editing; preserve every earlier migration byte. Frozen hot metadata:
	directories, logical_tables, fields (stable-ID tombstones), form_views,
	table_field_dependencies, table data/dependency revisions, app structure
	revision, scoped resource registration, new operation kinds and controlled DDL.
	Metadata/config/layout may use JSONB; business rows have physical typed columns.
	Dependency registration is real local authoritative storage with explicit
	availability ports, not a guessed Flowable status. An authoritative available
	registry can legitimately be empty when no flows exist; absent/unavailable
	adapters must not fabricate an empty registry and block destructive Save.
- `infra/runtime/roles.sql`/`cold-roles.sql` append explicit new capabilities,
	preserve old statements. No runtime owner transfer or broad schema CREATE.
	New structural audit needs hot+cold compatible allowlisted summaries; existing
	B5 summary restrictions remain intact. No data/default/layout values in audit.
Extra CI reuse requirement discovered from the fixed B2 source: its storage tests
require the dedicated `WEAVEOS_B2_TEST_DATABASE_URL` private socket fixture,
which PR26 does not contain. To retain its original regression tests, the lead approves V030-013 ownership
of only the PR25 B2 lifecycle sections in `.github/workflows/ci.yml`,
`infra/acceptance/run.mjs` and `tests/foundation/b2-ci-database.test.mjs`
(the exact path was verified from fixed PR25). This is not
authority to change the ongoing PR26 CI repairs or original acceptance gates.
No production key is generated or installed by this contract. Confirmation signing
keys are dedicated, injected, and versioned by keyId; no real production
credential creation or configuration is authorized.
No go.mod/go.sum/pnpm edits or dependency additions are approved by this contract.
## 实施、验收与接线放行
技术 owner 为 V030-013；执行任务 01a100ea-61ec-7389-9064-ab06a862c00f，当前 turn 01a100fb-bdf2-7382-9df9-28be1d037a52。执行者据本页修正仓库 proposal；本页读回后由主负责人释放 HTTP 与前端消费。未回读的代码接口不作为已交付 API。
沿源合同 §8 和任务 PRD 执行真实 PG／受限角色 RED／GREEN。额外覆盖：新表单原子创建失败不留孤立对象、现有表多视图、span 默认与 1–12 边界、纯布局不改数据、规范化默认值幂等等价。实际测试结果、代码 SHA、耗时／锁／容量限制与未跑项另记，当前不得标完成。
代价与边界：同 app 配置门禁和同表 DDL 锁会串行化相应操作；ACCESS EXCLUSIVE 会阻塞同表读写，不能称低停机。完整记录查询、字段数据权限与 Flowable 不在本包。旧迁移逐字节保留，热 00007／冷 00004 使用前再次检查未占用，既有门禁不削弱。没有生产 DDL／授权、真实凭据创建、main merge／deploy 放行。
</content>
</page>
