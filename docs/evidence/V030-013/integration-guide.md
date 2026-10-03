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
