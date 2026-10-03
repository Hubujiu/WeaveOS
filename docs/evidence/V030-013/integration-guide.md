# V030-013 available integration and isolated test configuration

Current published API baseline: bf4956394581979f3371a8840a95f3c59166b801,
draft PR27, stacked on PR26 b4c0fb3927d71a44c9a299c9c0bc241d42663893.
V013 owns applications shared dispatcher/types/store/transactions, BFF config,
OpenAPI/errors and allocated migrations/roles. V015 owns its four core modules.
No main merge, deployment, production role/DDL/data or new production credential
is authorized by this guide.

## Available product endpoints

Relative to `/api/v1/applications/{appId}`: GET structure; POST/PUT directories,
tables and forms; GET form definition; POST definition preflight; PUT definition;
GET member-candidates and department-candidates. OpenAPI gives exact paths/DTOs.
Owner/Bootstrap definition management is checked against actual Session and
registered resources. Ordinary menu access does not inherit child management.
Expected-actor header is an optional negative guard; it never grants permission.
Preflight is read-only. Save executes real typed PG DDL with metadata/layout/
audit/operation in one transaction. Unknown submission recovers the original key
through GET `/api/v1/application-operations/{operationId}`.

## Isolated BFF test configuration

Use the existing authentication/Redis settings for an owned isolated database;
apply hot7/cold4 and reviewed hot/cold roles to their respective databases first.
Definitions need four explicit settings together:

| Variable | Exact shape | Example test setting |
| --- | --- | --- |
| WEAVEOS_DEFINITION_HMAC_KEY | strict standard base64, decoded length at least32 bytes | generated ephemeral key below |
| WEAVEOS_DEFINITION_KEY_ID | 1–16 ASCII letters/digits/underscore/hyphen | v013_test |
| WEAVEOS_SCHEMA_LOCK_TIMEOUT_MS | positive signed32-bit integer milliseconds | 1000 |
| WEAVEOS_SCHEMA_STATEMENT_TIMEOUT_MS | positive signed32-bit integer milliseconds | 5000 |

Missing any setting while another is specified fails startup. All absent keeps
legacy startup/definition reading, but definition mutations fail closed because
transaction limits were not configured. Confirmation token lifetime is10 minutes,
bound to actual actor/app/table/view, versions, data/dependency and normalized plan.
The definition key is separate from the existing audit key and is solely for
schema confirmation tokens. It is not a database actor assertion or Session key.

This snippet generates only an ephemeral test environment file in `/tmp`, mode
0600, and prints no key. Do not upload/source secrets from untrusted files.

```bash
python3 - <<'PY'
import base64, os, secrets
path = '/tmp/weaveos-v013-definition-test.env'
fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(fd, 'w') as output:
    output.write('export WEAVEOS_DEFINITION_HMAC_KEY=' + base64.b64encode(secrets.token_bytes(32)).decode('ascii') + '\n')
    output.write('export WEAVEOS_DEFINITION_KEY_ID=v013_test\n')
    output.write('export WEAVEOS_SCHEMA_LOCK_TIMEOUT_MS=1000\n')
    output.write('export WEAVEOS_SCHEMA_STATEMENT_TIMEOUT_MS=5000\n')
PY
source /tmp/weaveos-v013-definition-test.env
```

This guide does not create/configure any production key or deploy a service.
Test fixtures inject synthetic keys, real Redis Session and actual restricted PG
roles; standalone BFF composition now initializes its own reviewed role source
instead of relying on another test package having run first.

## Shared V015 delta

Actually read accepted ADR revision11:08:25.612Z and frozen PRD09:44:31.464Z;
full source in v015-shared-source-readback.md. §8 record/query/draft is frozen.
§10.1 rejects new DB actor HMAC credentials: Redis Session+BFF is the end-user
boundary; auth_app is the trusted service role. Typed DML validates real table/
active-column registration, canonical values, protected system columns and CAS,
with no generic SQL/DDL. BFF performs current Session/CSRF/grants/fence in the same
transaction. V015's older credential proposal is superseded on this point.

The parent's renewed implementation instruction and ADR §8.10 sole-owner
allocation cover registered hot8 `00008_app_records.sql`; actual scans found it
unoccupied. Hot7 remains byte-identical. ADR11–12 was subsequently fetched and
freezes runtime/history/quick-search; Q36 extraction belongs to V015. Neither
owner-only definition endpoints nor this checkpoint provides record HTTP yet.

## Shared ports available in the hot8 checkpoint

Apply current hot migrations through8 and reviewed roles to an isolated DB.
The following implementations are actual restricted-PG ports, not runtime mocks:

```go
applications.Application.BeginRecordWrite(ctx, principal, appID, viewID,
    applications.RecordWriteOptions{OperationID: operationID, Kind: kind,
      Fingerprint: fingerprint, LockTimeout: limits.LockTimeout,
      StatementTimeout: limits.StatementTimeout,
      SourceGuard: sortedSourceLocker, Authorize: trustedActionPolicy})
// Returned RecordWrite exposes Tx, Context, Replay, Claim, Complete, Commit,
// Rollback. Replay must precede Claim/new schema/record/draft/fence checks.
// Claim invokes the required injected policy; nil fails closed.

appstructure.RecordDML{}.Insert(ctx, tx, table, create, selectedIDs)
appstructure.RecordDML{}.LockHeader(ctx, tx, table, recordID)
appstructure.RecordDML{}.UpdateCAS(ctx, tx, table, edit, selectedIDs)
appstructure.RecordGate{AppID: appID, TableID: tableID, ViewID: viewID}
appstructure.RecordFence{AppID: appID, TableID: tableID}
appstructure.RecordAudit{Context: write.Context(), OperationID: operationID,
    BeforeRecordVersion: oldRecordVersion, Metadata: actualRequestMetadata}
```

Neutral shapes RecordTable/RecordCreate/RecordEdit/StoredRecordHeader/
RecordMutationResult exactly mirror V015's fixed field order/types. V015 can use
ordinary Go conversions inside its own adapter, preserving its exclusive package:
`appstructure.RecordTable(table)`, `appstructure.RecordCreate(in)` and
`apprecords.StoredHeader(header)`. No V015 core files are copied here. DML ignores
the supplied Namespace and derives appdata physical identifiers solely from actual
app/form/table/active-field registration. Canonical values are checked in Go and
in the finite DB capability. Shared native errors are appstructure.Error with
registered code, applications.ErrMissing/ErrResourceInvalid, or session.ErrUnavailable;
the consumer must preserve those identities/codes when mapping HTTP results.

RecordContext holds actual live actor/bootstrap/app owner, form→table/schema facts
and complete enabled-group grants with field IDs, loaded under the app/policy gate.
Injected policy is the entry/action boundary; the V015 writer must still check
complete row/field policy, CAS, references and fence inside this caller transaction.
No ordinary member is routed through manager-only BeginManagerWrite.

Operation kinds include record.create/edit and draft.create/update/discard;
confirmed record result is exactly operationId/id/recordVersion/schemaVersion/
createdAt/updatedAt. Draft ledger uses only operationId/id/draftVersion, never
stored payload. Discard ledger accepts204 internally; external204 has no body.
Active actors can recover their own minimum confirmation after grant revocation;
confirmed replay skips new-source/action-policy checks and old CAS.

Minimum RecordAudit contains IDs/versions/changed field IDs and actual requestId,
no raw values. It is not the newly frozen record save history; history delta and
read/runtime/search HTTP are still the next shared/consumer integration stage.

Hot8 source tables member_sources/department_sources/member_department_sources
and singleton reference_source_revision are real. Actual SQL source insert/rename/
status/delete and multi-department changes synchronize them transactionally;
deleted UUIDs retain labels and cannot be reused. Ordinary display/candidate ports
and query hydration must be wired using the frozen minimum DTO, not by opening
the owner-only candidates. Do not interpret missing source rows/counters as zero.

Real hot8 fresh upgrade, native DML/system/CAS/NULL capability guards, grants,
multiple departments/tombstones, unchanged20-statement grant budget and FK cleanup
pass in record-shared-fresh-green.txt. Real gate/empty+pending fence/minimum audit/
operation commit/replay pass in record-ports-green.txt. Full regression/CI still
needs the independently registered old member oracle and deploy source pin changes.

## Runtime and shared RR read port (after249dc59)

`applications.Application.BeginRecordRead(ctx,principal,appID,viewID)` returns
`(pgx.Tx, applications.RecordContext, error)` with caller-owned read-only RR;
live active actor/auth-version, real enabled catalog and registered form/table,
complete current form grants and masks stay in that snapshot. Context now adds
`ViewVersion` and `Layout json.RawMessage`; writes use the same metadata loader.
This resolves facts only; the consumer must enforce actual menu/action policy.

`appstructure.ResolveRecordAccess` is injected on `Application.RecordAccess`:
`func(context.Context, pgx.Tx, applications.RecordContext) (RecordAccess,error)`.
`RecordAccess` has MenuEnter/Create bool, Read/Edit/History scopes string
(none/own/all), Fields map[fieldID]FieldAccess; FieldAccess has Read/Edit/History
scopes and Create bool. The V015 policy consumer owns tuple evaluation; this
shared port must receive trusted facts and the caller's same snapshot, not body.
Runtime excludes History from the public field access object. Owner/Bootstrap
uses existing live manager identity over real registered resources. Ordinary
subjects require this injected resolver; absent resolver fails503.

`ProjectRuntime` returns the frozen safe RuntimeView, union create/read/edit
fields, permitted constant defaults, whitelisted kind inputs including minute,
scope-covered query affordances and pruned layout/empty groups. Actual owner GET
`/forms/{viewId}/runtime` now passes Session/expected-actor/resource/not-ready,
restricted-PG and canonical-minute/default checks. Ordinary HTTP permission
matrix waits for the V015 resolver; projection tests are not that acceptance.
Source read SQL/hydration remains V015-owned per ADR§13; no shared duplicate.

## Same-Tx history wrapper and owner history HTTP (hot9)

Use `appstructure.RecordHistoryDML{History:appstructure.RecordHistoryStore{},
Origin:"ordinary"}` instead of the plain RecordDML adapter in the composition.
Its exact Insert/LockHeader/UpdateCAS signatures and neutral structs are unchanged,
so the existing V015 ordinary struct conversions still work. Insert captures
all actual SQL field values, including omitted DB constant defaults, under the
real table gate. Edit captures canonical selected old values under row/table
locks and compares the real canonical mutation; no same-value delta/event.
History writer absence fails closed. Native capability errors retain identity.
No Begin/Commit in this wrapper: event/delta/record/draft consumption/minimum audit
and operation belong to the caller's one transaction. Storage failure aborts it.
`RecordHistoryWriter.Append(ctx,pgx.Tx,HistoryMutation)` is the neutral injection
port; origin task_save and opaque task reference exist for future trusted task
Save, without authorizing task endpoints/workflow behavior in this slice.

Hot9 adds independent record_change_events/record_change_values and option-label
tombstones. Event-time kind is immutable; field removal retains existing field
metadata tombstone. Data-grant removal dependency kind is `data_grant`, with
resourceId equal to actual persisted grant ID; explicit revoke is required, and
mask mutations advance table dependency revision. Previous hot1–8 stay unchanged.

GET `/forms/{viewId}/records/{recordId}/history` is wired through real Session,
expected-actor, registered metadata and same readonly RR. The existing injected
RecordAccess must include History and per-field History scopes, resolved by V015
from current complete tuples. Current row read404; no current history403; selected
field read AND history applied before LIMIT/cursor. Owner/Bootstrap has actual
resource-bound removed-field access. Cursor uses existing configured Redis and
namespace, binds actor+SessionRef+app/table/form/record+policy/schema+pageSize;
opaque10-minute server cursor, no business values or credentials. Changed policy
or schema requires a fresh cursor, after current permission checks. Actual owner
HTTP/pagination/tombstone tests pass. Ordinary matrix awaits V015 policy consumer.

Formal POST/PATCH record/search/draft HTTP remains the V015 service composition
stage. Persisted hot8 draft minimum is exactly operationId/id/draftVersion (three
keys); source13 accepts the fixed46cf01b guide. Worker integration must retain this
shape or obtain a coordinated additive migration; full Draft values never enter
operation results. Record minimum stays the exact six frozen keys.

Schema Save produces metadata rule audit only, no per-row save deltas/version
increments. The frontend confirmation must explicitly state that lossy previous
row values cannot be restored from record save history (ADR11.4); backend retains
its frozen impact counts/kinds rather than inventing a new restoration API.

## Account-source preservation correction (hot10)

Existing accepted FR002/Q13 permits254 Unicode account characters. A real product
CI registration test exposed the hot8 source label100 mismatch. Hot10 preserves
full labels/tombstones on databases that applied earlier draft8. The still
unmerged/unreleased hot8 draft seed is also corrected to254 so an existing hot7
account of that length can upgrade through8; its original fixed46cf01b source
remains recoverable. This supersedes earlier statements that draft8 bytes remain
unchanged; released migrations1–6 and fixed dependency commit identities do not
change. Consumers should take the updated source/migration checkpoint, without
editing goose history. Upgrade7→10 and empty1→10 both passed in owned isolated
PostgreSQL; full race/vet and279 governance/foundation/contract tests passed.
No production role/schema/data action was performed.
