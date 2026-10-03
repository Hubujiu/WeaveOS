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

Shared incremental hot8 numbering and exact added paths have been submitted to
the lead for registration before edits. Hot7 remains byte-identical. Value history,
runtime DTO/reference displays, quick-search and public data-grant dependency
expression still require their exact technical review. Neither owner-only
definition endpoints nor this baseline provides ordinary record HTTP yet.
