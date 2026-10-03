# V030-015 isolated core RED/GREEN, 2026-10-03

Source: [frozen V030-015 PRD](https://app.notion.com/p/3ee2f5a9e6488116be71c0a0c332a27a),
[accepted V030-015 ADR §8](https://app.notion.com/p/3ee2f5a9e6488163a144fba5f55e3c14),
V030-013 authoritative ADR. This record covers the **exclusive package core**,
not the real app HTTP/API or full product acceptance.

## Environment

- Independent worktree `task/V030-015-records-query-drafts`; original PR26 base
  `9b89e8e928aedf30df235492fe89bc52021f6fe9`; prior proposal commit
  `2da21b2a775071e760a03c6572b5ac712792a5d5`. V030-013 remote later
  advanced to `4dcad40f09d5d20c2d2d9f97a12c7e41b4630bed`; V015 did not
  merge it or edit its shared files.
- Isolated local container `weaveos-v015-pg`, PostgreSQL `18.6
  (Debian 18.6-1.pgdg13+2)`, database `weaveos_v015`, trust auth only on local
  port 65432. Tests create temporary tables, no production data or migrations.
- Go 1.27.1 at `/workspace/.weaveos-tools/go/bin/go`; GOCACHE in `/tmp`.
- Tests execute as `weaveos_owner`. They **do not** prove `auth_app` least
  privilege, actual migrated `applications` schema, Session/CSRF or BFF routing.

## Target RED before each implementation

| Target test | Observed failure, exit 1 | Requirement |
| --- | --- | --- |
| `go test ./internal/appaccess -count=1` | `TestCompleteGrantTuplesAndCreateEmptyMask`: scopes incorrect; `TestNoReadableFieldNoRowAndRealResourceGate`: owner gate incorrect | ADR §8.2, full tuple OR, empty create mask |
| `go test ./internal/appquery -count=1` with local PG URL | `TestNestedTypedConditionsAndSetEqualityInRealPG`: invalid record search criteria | ADR §8.6, typed nested SQL and set equality |
| `go test ./internal/appdrafts -count=1` with local PG URL | `TestRealPGPartialDraftRevocationAndExactConsume`: invalid draft input | ADR §8.7, partial payload/mask/consume |
| `go test ./internal/apprecords -count=1` with local PG URL | `TestTypedPGCreateEditCASFenceAndRollback`: record dependency unavailable | ADR §8.5/8.9, typed row CAS/fence/audit |
| `go test ./internal/appquery -run TestPGFullProjection -count=1` with local PG URL | `TestPGFullProjectionChangesOnlyWhenActualAuthorizedRowsChange`: invalid authorized projection stream | ADR §8.1, full authorized projection fingerprint |
| `go test ./internal/appdrafts -run TestRealPGOwnerCursorDraftList -count=1` with local PG URL | `TestRealPGOwnerCursorDraftList`: invalid draft input | ADR §8.3/8.7, indexed owner cursor list |

The target tests were written against explicit fixture rows/grants and method
behavior. Stubs compiled and produced the above assertion failures; missing
imports, missing database and syntax errors were not counted as RED.

## GREEN observed

Command, from `services/bff` (the test URL targets the isolated local PG):

```sh
WEAVEOS_TEST_DATABASE_URL='postgres://weaveos_owner@127.0.0.1:65432/weaveos_v015?sslmode=disable' \
GOCACHE=/tmp/weaveos-v015-go-cache GOPATH=/workspace/.weaveos-tools/gopath \
/workspace/.weaveos-tools/go/bin/go test -race \
./internal/appaccess ./internal/appquery ./internal/appdrafts ./internal/apprecords -count=1
```

Latest rerun exited 0 at 2026-10-03 approximately 10:55 UTC:

```text
ok github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess 1.027s
ok github.com/Hubujiu/WeaveOS/services/bff/internal/appquery 1.075s
ok github.com/Hubujiu/WeaveOS/services/bff/internal/appdrafts 1.067s
ok github.com/Hubujiu/WeaveOS/services/bff/internal/apprecords 1.094s
```

`go vet` on the same four packages exited 0. The test facts are narrow:
`appaccess` checks complete tuple/row/field calculations with trusted
in-memory facts; `appquery` executes compiled SQL against temporary typed
columns and hashes ordered PG JSONB rows without buffering all rows;
`appdrafts` exercises incomplete values, revoked-field masking, partial
updates, no-op, stale consume, rollback and cursor pagination in PG;
`apprecords` exercises typed row create/edit, real PG row locks/CAS, a local
pending-fence table, audit insertion and no partial mutation. The test
authorization/gate/audit adapters are fixtures, not the production ports.
An additional real PG test uses two distinct connections and a persistent
isolated test schema: both writers race the same recordVersion, exactly one
commits/version advances, the other returns conflict, and one audit row is
committed. The gate fixture also rechecks current schemaVersion after locking;
changing the schema row blocks a stale write. This concurrency assertion was
added as verification after the basic write path GREEN; it is not represented
as a new target RED. Later `appquery` additions execute row and field masked
SQL in real PG, check all/own COUNT bind alignment, reject an own-only field
in an all-row filter/sort before executing SQL, guard safe arbitrary OFFSET,
and measure streamed JSONB bytes. The first all+own COUNT assertion failed
with a missing `$1` bind when the actor was used only in projection; separate
COUNT/page bind vectors corrected it and the rerun above passed. These are
core checks, not a live grant-loader or full Q36 integration claim.

## Isolated scale core, including failed capacity cases

[Scale report](scale-core-report.md) contains the reproducible harness,
raw logs, EXPLAIN plans and timing/byte matrix. It ran real local PG18.6
10k/100k/1m typed TEMP tables with C=1/20/200 sequential rehash probes.
The unfiltered one-million-row C=200 case **timed out at the unchanged
10-minute test deadline in both whole-matrix and isolated runs**. At one
million rows, C=1 took 3.24–3.56 s, C=20 took 45.3 s, C=200 at 10% selectivity
took 63.6 s, and C=200 at 0.1% took 0.531 s. This is an explicit capacity
finding for frozen algorithm A, not a product acceptance pass.

## Follow-up controlled DML and compact-A experiment RED/GREEN

After the lead requested a controlled typed-DML seam and an **unselected**
compact-A experiment, new target tests were run RED before their behavior was
implemented:

| Target | Real observed RED, exit 1 | GREEN scope |
| --- | --- | --- |
| [typed-dml-port-red.txt](typed-dml-port-red.txt) | `TestTypedPGCreateEditCASFenceAndRollback`: missing port still executed a real PG business INSERT and returned nil | `Writer` now requires `TypedDML`; direct SQL exists only in owner-role test fixture; real CAS/fence/rollback/concurrency pass |
| [compact-candidate-red.txt](compact-candidate-red.txt) | `TestCompactCandidateAgainstCompletePGOracle`: a visible text mutation changed the independent full P but stub compact signature did not | Test-only compact candidate detects all enumerated P changes under its stated version/schema/source invariants; deliberately unversioned writer shows false negative |
| [compact-cache-red.txt](compact-cache-red.txt) | `TestCompactExperimentCacheScopeAndSingleFlight`: same canonical key recomputed 200 times | Test-only 32-entry, 5-second digest/count/revision cache isolates actor/mask/criteria/revision and runs one compute for 200 concurrent calls |

The final isolated recheck at 2026-10-03 11:37 UTC ran `go test -race -count=1`
on all four exclusive packages against local PostgreSQL 18.6; all four exited
0. `go vet` for the same packages, `check-tasks`, `verify-repo`,
`git diff --check` and `gofmt -l` also exited 0. These checks do not replace
the unrun product gates below. The
[compact-A report](compact-A-report.md) records real PG same-key/distinct-key
costs and the proof obligations; the candidate is **not** installed in the
Q36 lifecycle. The [quick-search PG probe](quick-search-cost.txt) confirms
selected ASCII-only literal semantics in isolated SQL and shows the existing
text BTree still produced a ~5-second Seq Scan over 1m rows. No runtime
quick-search route/index or history exposure route was implemented.

## Unrun product gates and exact reasons

- V030-013 now has hot7 typed tables, the field normalizer and manager
  definition lifecycle, but no V015 non-manager BeginRecordWrite, persistent
  data grant migration, record/draft operation kinds, restricted typed DML
  capability or BFF routes. PR26 has menu-only grant constraints. Shared
  additions and precise path ownership are in
  [shared integration proposal](shared-integration-proposal.md).
- `appquery` has the typed compiler, permission-conditioned SQL fragments,
  safe OFFSET and streaming fingerprint primitive, but no shared Q36 Redis
  context lifecycle, live COUNT/page executor, same-RR reference resolution
  or policy revision handling. Core-stage scale is measured above; no timeout
  was enlarged. The required full-chain matrix remains NOT RUN.
- The package tests do not exercise actual `auth_app` roles, Session, origin,
  CSRF, unknown COMMIT, shared operation replay, source hooks, task-scoped
  authorization, real Flowable fence, OpenAPI or cross-browser API. These are
  **NOT RUN**, not GREEN or silently skipped. The parent integration plan
  must supply the shared ports and run the full independent matrix.
- At the original core checkpoint quick search and old/new business history
  were separate proposals. The lead later selected quick-search matching and
  a default-deny `data.history` direction, documented in the shared delta;
  the routes remain unimplemented and history exposure needs further freeze.
  No application old/new values are written by this core; the current
  `apprecords.Audit` port accepts only minimum summary metadata.
