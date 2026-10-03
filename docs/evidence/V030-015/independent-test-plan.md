# V030-015 independent RED/GREEN and capacity plan

Status: **CORE CONTRACT FROZEN; PARTIAL CORE TESTS RUN**. Lead froze the
2026-10-03 V030-015 Notion PRD/ADR §8 and released record/query/draft core.
Expected behavior comes from that decision,
the V030-015 PRD/ADR, V030-013 authoritative field contract, accepted
ADR-002 and independently written fixtures. Current handler output is
not an oracle. The source readback/release was completed before target tests.

## Fixture and RED order

Use isolated PG18.6 with V030-013's final hot/cold migrations and restricted
auth_app role; real Redis for Session/query context. Never point tests at
production or real personnel data. Set the test clock and app/actor IDs
deterministically. Retain each test source hash and actual command/output
before the corresponding implementation. A missing import/connection,
syntax failure or fake guard is not a valid product RED.

1. Freeze final Notion DTOs and shared ports; write contract tests for
   body/response shapes and strict limits, then observe target RED at the
   unavailable record/draft/search routes and missing grant/fence behavior.
2. Write restricted-role PG tests for data grant storage and same-form
   field FKs, current actor/session, real table-row CAS, record audit and
   operation ledger; observe target RED before each smallest implementation.
3. Write query result oracle from explicit rows and grants, independent of
   the compiler. Exercise complete projection and relevance with one
   RR snapshot. Observe RED before appquery generalized lifecycle code.
4. Write draft partial-update, revocation and consume tests, including
   concurrent versions and unknown commit. Observe RED before appdrafts.
5. Write real source hook, multi-department and pending-command fence
   integration tests after those authoritative owners supply ports; no
   no-op substitutes. Add actual HTTP/cross-browser and scale runs.

Each phase records command, environment, SHA, UTC time, exit code,
failed assertion, corresponding requirement, GREEN command/result, and
any failure/skip/not-run. Mock-only and pure normalization tests cannot
certify PG/role/transaction/cross-browser behavior.

## Deterministic semantic oracles

All UUIDs below are distinct and canonical. Form F1 and F2 share one table
in app A; F3 belongs to app B. Owner O, ordinary U, other V and trusted
Bootstrap R each have independent real Sessions. Fields X,Y are text,
M is multi-select with options a,b, N is decimal and T is datetime.

| ID | Given / action | Independent expected outcome |
| --- | --- | --- |
| AUTH-01 | U has create-only capability at platform level; O owns app A | U cannot manage/read A unless a concrete F1 grant exists; O manages A, not B |
| AUTH-02 | Group G1 grants U edit own X; G2 grants read all Y | U edits X only on a row with immutable createdBy=U; cannot edit Y or V's X; editing a row never changes its own identity |
| AUTH-03 | G1 grants U read all X; G2 read own Y; query all rows sorted/filtered by Y | Entire query 403 before Y values are read; it must not return only U's rows or a count that varies with Y |
| AUTH-04 | U has read own X, no all read grant; filter own rows on X | Allowed; COUNT and page include only U-created rows; V-created rows never contribute |
| AUTH-05 | U has data.create on F1 with empty mask and X has valid server default | POST with empty values creates using default; POST supplying X returns 403; result contains no business values |
| AUTH-06 | U can read F1 but no F2 grant, same physical row | F1 GET can show authorized values; F2 route cannot borrow F1 grant; app B row is inaccessible |
| AUTH-07 | U loses read for one field after seeing a row; GET/history/search again | Current mask applies immediately; old operation returns only minimum confirmation, never former values |
| AUTH-08 | No form entry versus unreadable/cross-app record | No form entry 403; unreadable/cross-app record 404; no existence leak |
| FIELD-01 | Definition layout omits X, or browser hides X, but U has read X | Complete query projection still includes X; changing off-page X makes old context QUERY_CHANGED |
| FIELD-02 | A row has no readable business field for U | It contributes zero to COUNT and appears on no page |
| FIELD-03 | U holds read all X from two groups and edit own X in one | Grant OR uses complete tuples; field/action/scope cannot form a synthetic edit-all grant |
| QUERY-01 | Multi-select stored [a,b]; filter eq [b,a,a] | Match; set order/duplicates do not change equality; neq [a] matches; neq nonnull does not match SQL NULL |
| QUERY-02 | Nested AND/OR combining X eq, N gt and T lte | Server typed SQL returns the exact hand-enumerated row IDs/count; text gt or text sort is rejected |
| QUERY-03 | N values tie under ascending sort, including null | Stable ID breaks equal numeric/time ties; nulls are last; page 1, a deep page and beyond-total page have stable contents |
| QUERY-04 | Data revision advances after a write to an unrelated nonmatching row | Selected A recomputes P but digest equals prior; no QUERY_CHANGED; context revision advances by CAS |
| QUERY-05 | Matching row changes authorized Y off current page or sort key changes | P changes and old context returns QUERY_CHANGED; explicit refresh without old token starts page 1 |
| QUERY-06 | Source display changes for a referenced matching row | P changes under one RR snapshot; old query context changes even if stored UUID is unchanged |
| QUERY-07 | Policy grant changes while record data revision is stable | Live policy is checked before page; stale unauthorized field never serialized; changed authorized P is not silently reused |
| QUERY-08 | Concurrent writer commits around query snapshot | COUNT, full fingerprint, page and registry display all reflect one RR snapshot; no mixed before/after response |
| RECORD-01 | F1 and F2 address the same physical record; two editors use same expected version | Exactly one edit commits/version advances; other gets record CAS 409; both views observe same row/version |
| RECORD-02 | New typed value/default invalid, field tombstoned or schemaReady=false | 400/field error or SCHEMA_NOT_READY 409 as specified; no row/audit/operation partial commit |
| RECORD-03 | Committed response is lost; actor retries exact body/key after grant revoke | GET original key returns minimum confirmed MutationResult; no duplicate version/history; no old field values |
| RECORD-04 | Same actor/key with changed body or route | OPERATION_CONFLICT 409; different actor cannot claim or read another actor's key |
| FENCE-01 | Fence port absent/unavailable | 503; no row or history write |
| FENCE-02 | Real local authoritative fence table has no pending row | Ordinary edit may pass other auth/CAS checks; this is not a hardcoded allow guard |
| FENCE-03 | Same record has pending approval command | Edit is fenced 409 even for owner/Bootstrap; no row/history mutation |
| TASK-01 | Future task-scoped approver may edit only amount, ordinary data.edit absent | Trusted future task port may authorize amount independently; manager-only field remains blocked; Save changes row/history but does not complete task |
| DRAFT-01 | U creates incomplete draft with decimal text "1." and omitted required X | Shape/permission/1 MiB raw checks pass; no formal value validation, row/revision/audit/flow side effect |
| DRAFT-02 | Draft PATCH changes X, omits Y and removes Z | Y persisted exactly; X replaced; Z removed; change/removal overlap rejected; empty update does not bump draftVersion |
| DRAFT-03 | Z permission revoked after draft save | GET omits Z value and returns FIELD_PERMISSION_REVOKED conflict plus hasConflicts; stored Z survives; submit blocks |
| DRAFT-04 | U explicitly removes now-inaccessible Z via removeFieldIds | Own draft loses Z without exposing its prior value; other draft keys remain; stale draftVersion conflicts |
| DRAFT-05 | Schema or base record changed | Draft retains original binding/payload, reports SCHEMA_CHANGED/BASE_RECORD_CHANGED with fieldId=null; no silent rebase or delete; new draft requires explicit action |
| DRAFT-06 | Whole F1 resource access revoked | GET draft is 403; ownership alone cannot bypass form auth |
| DRAFT-07 | Formal submit consumes exact draftVersion in same row/history/operation tx | Rollback preserves draft; successful commit removes exact version; later saved version cannot be silently consumed |
| HIST-01 | Edit X and secret Y in one event; U currently may read only X | History returns X old/new only; never Y value, ID, or hidden change count |
| HIST-02 | Event changes only unreadable Y | Event absent before LIMIT/cursor; U cannot infer actor/time/version from it |
| HIST-03 | Read grant to X revoked after one history page | Next page/replay rechecks current rights; no old X values; row revoke returns 404 |
| HIST-04 | Ordinary edit and future node Save | Canonical old/new deltas, actor/version/operation same tx as row; global auth/personnel audit summary contains no values |
| REF-01 | Member in two departments moves one membership | Registry projection retains both actual associations; never chooses first; same-name new account cannot inherit tombstone ID |
| REF-02 | Deleted member/department has historical reference | Old record keeps UUID/last display; new selection rejects tombstone; source change before COUNT/LIMIT when predicate uses source |

Task-scoped TASK-01 is a future integration expectation; it remains NOT RUN
until the flow owner freezes and supplies a real task authority. Current
V030-015 code cannot claim it by stubbing task state.

## Failure injection and concurrency

For each formal write inject failure after typed row, after first history
value, after draft consume, after operation result and at COMMIT response.
Verify with a separate connection that all committed components agree or
all roll back. Treat a COMMIT error without explicit rollback proof as
UNCONFIRMED, preserve the original key/body, and query the durable result.
Run concurrent same-key and same-recordVersion writers through distinct
PG sessions. For revocation races, lock ordering must be PR21 revisions,
live AccessForWrite dependencies, app/policy, table gate, record/fence,
history, operation result. Test both revoke-first and write-first.

History SELECT tests use a restricted role with no blanket SELECT on raw
record_change_values; verify no global authentication event exposes
old/new values. Draft list must batch conflict evaluation for one page,
not issue one unbounded query per saved draft.

## Real capacity and memory matrix for selected A

Seed deterministic 10k, 100k and 1m typed records in an isolated database.
Vary matching fraction (roughly 0.1%, 10%, 100%), all versus own grants,
0/1/20 filter leaves, eq/neq/range/OR, sort indexed/unindexed, nullable and
high fanout references. Use 20/100 page sizes and offsets 0, 50k, 500k
where row counts permit. Use 1/20/200 active contexts across multiple
Sessions, preserving the existing per-Session Q36 context limit where it
applies. Scenarios:

1. Initial full fingerprint and first page.
2. Same revision page change with saved total.
3. One unrelated write, then each active context revalidates.
4. One related off-page value/order change, then revalidation.
5. Burst writes and source display fanout 1/1k/100k.
6. Policy-only change and schema change with unchanged record data.

Run repeated warm and cold measurements with seed and settings recorded,
ANALYZE before plans, and EXPLAIN (ANALYZE,BUFFERS) for full projection,
COUNT and page separately. Capture p50/p95/p99, PG buffer/temp I/O, sort
spill, index size, WAL/write amplification from restricted history, DB
process RSS, Go RSS/heap peak, bytes streamed, lock wait/deadlocks and
Redis context bytes/CAS behavior. Compare one and 200 contexts in both
latency and aggregate DB work. Confirm application memory stays within
O(B*W+p*W+F), with B a fixed bounded batch, while PostgreSQL sort/temp
usage is reported separately. OFFSET depth is measured, never called O(1).

No SLA, new timeout, journal retention, record deletion or draft expiry is
inferred from results. If full projection at 1m/C=200 is unacceptable,
deliver exact plans and measurements to the lead for a new algorithm
decision; do not increase timeouts to conceal the cost. A remains the
selected design until the source decision changes.

## End-to-end release evidence after module GREEN

Run actual HTTPS Session/CSRF and OpenAPI 3.2.1 response validation against
the real PG-backed BFF. Exercise the existing unified Table queryVersion,
hidden-column state, explicit page-one refresh and draft conflict UI in
Chromium, Firefox and WebKit. A fixture-only browser green cannot replace
the real API run. Report PR/head CI separately from local results. No
main merge, production role/DDL, deployment or actual account deletion is
authorized by this test plan.
