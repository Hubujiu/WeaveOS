# V030-015 isolated core RED/GREEN, 2026-10-03

Source: [frozen V030-015 PRD](https://app.notion.com/p/3ee2f5a9e6488116be71c0a0c332a27a),
[accepted V030-015 ADR §8](https://app.notion.com/p/3ee2f5a9e6488163a144fba5f55e3c14),
V030-013 authoritative ADR. This record covers the **exclusive package core**,
not the real app HTTP/API or full product acceptance.

## Environment

- Independent worktree `task/V030-015-records-query-drafts`; original PR26 base
  `9b89e8e928aedf30df235492fe89bc52021f6fe9`; prior proposal commit
  `2da21b2a775071e760a03c6572b5ac712792a5d5`. V030-013 remote still
  `269c77407375e8ff983861484a8548d03ba42a5a` at 2026-10-03 10:00 UTC.
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

Final rerun exited 0 at 2026-10-03 approximately 10:14 UTC:

```text
ok github.com/Hubujiu/WeaveOS/services/bff/internal/appaccess 1.014s
ok github.com/Hubujiu/WeaveOS/services/bff/internal/appquery 1.052s
ok github.com/Hubujiu/WeaveOS/services/bff/internal/appdrafts 1.067s
ok github.com/Hubujiu/WeaveOS/services/bff/internal/apprecords 1.090s
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
as a new target RED.

## Unrun product gates and exact reasons

- V030-013's fixed branch still has no released integrated form/table metadata,
  full kind normalizer, schemaReady/table-gate port, persistent data grant
  migration, operation kind extension, or BFF routes. PR26 has menu-only
  grant constraints. Those shared files belong to its integration owner.
- `appquery` has the typed compiler and streaming fingerprint primitive, but
  no shared Q36 Redis context lifecycle, live COUNT/page executor, same-RR
  reference resolution or policy revision handling. No 10k/100k/1m and
  1/20/200-context capacity results exist; no timeout was enlarged.
- The package tests do not exercise actual `auth_app` roles, Session, origin,
  CSRF, unknown COMMIT, shared operation replay, source hooks, task-scoped
  authorization, real Flowable fence, OpenAPI or cross-browser API. These are
  **NOT RUN**, not GREEN or silently skipped. The parent integration plan
  must supply the shared ports and run the full independent matrix.
- Quick search and old/new business history are separate proposals awaiting
  review. No application old/new values are written by this core; the current
  `apprecords.Audit` port accepts only minimum summary metadata.
