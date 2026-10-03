# V030-015 · Q36 single lifecycle extraction evidence

Status: **extraction implemented and local regression green; record HTTP
consumer and product acceptance remain open**. The frozen source is
[V015 PRD](https://app.notion.com/p/3ee2f5a9e6488116be71c0a0c332a27a)
(last edited 2026-10-03 11:48:10 UTC) and
[V015 ADR §11–12](https://app.notion.com/p/3ee2f5a9e6488163a144fba5f55e3c14)
(last edited 11:49:12 UTC), fetched in full before code edits. The
[frozen plan](q36-extraction-plan.md) supplies the exact boundaries.

## Exclusive source changes

- `internal/querycontext/context.go`: the sole runtime Lua Create/Load/Advance
  store with immutable data, separate revision CAS, per-Session 20-entry LRU,
  30-minute idle TTL and original personnel key/JSON bytes. A domain policy
  validates canonical metadata and monotonic revision vectors. A direct
  Advance reads only the saved view discriminator before the Lua CAS; that
  read does not touch the LRU/TTL.
- `internal/querycontext/engine.go`: the sole RR old-baseline/full-projection/
  page-only/Redis-publication lifecycle and read receipt. The six-method
  Strategy supplies current access, criteria, revision, full authorized P
  and page SQL; neutral code does not compile personnel or record SQL.
- `internal/personnel/query_context.go`: compatibility wrapper retains
  `QueryContext`, `QueryRevisions`, constructor, method signatures, closed
  member/event criteria validator and personnel error identity.
- `internal/personnel/query_engine.go`: member/event strategy retains exact
  filter/range normalization, fixed personnel SQL and in-RR hydration, plus
  the 10-second read deadline and HTTP-facing search methods.
- `internal/personnel/query_write_guard.go`: uses the neutral saved-read
  receipt; its RC business transaction still locks personnel revisions before
  authorization, rechecks the verified vector and retries at most three
  times **before mutation**. Draft cleanup and actual COMMIT remain there.
  No other personnel source, source hook, role, migration, OpenAPI or BFF
  composition file changed.

## Real RED → GREEN checkpoints

| Target | Actual RED | GREEN evidence |
| --- | --- | --- |
| Old/new Redis bytes and tokens | [84c91c7 test+declaration](https://github.com/Hubujiu/WeaveOS/commit/84c91c7cc47af6560f72858db6c98e4623c058f4): neutral Load returned expired for an actual old-store token; [raw log](q36-context-red.txt) | [e266566 dual-store implementation](https://github.com/Hubujiu/WeaveOS/commit/e266566c01692be7109e061af120df58fecf8ef9): while historical and neutral implementations both existed, real Redis proved old→new and new→old Load/Advance plus exact `data`/`revision` bytes; [race log](q36-context-green.txt) |
| Uppercase UUID Session compatibility | After wrapper, old-accepted uppercase Session text was rejected; [raw RED](q36-uppercase-red.txt) | Neutral Prefix folds for the same original hash; [wrapper race log](q36-wrapper-green.txt) |
| Shared RR lifecycle/receipt | [2c768e9 test+declaration](https://github.com/Hubujiu/WeaveOS/commit/2c768e9fe88790e9017c294b2dca3b912b3dbccf): first real PG+Redis query returned `ErrInvalid`; [raw RED](q36-lifecycle-red.txt) | The same test passes new baseline, page-only reuse, irrelevant revision CAS, changed old-P rejection, deep valid page, auth-before-expiry and Redis advance only after RR receipt commit; [BFF-wide race log](q36-bff-full-race.txt) |
| Resource-aware revision policy | Real PG/Redis test required Policy.Forward to receive actual `fixture` view; blank view returned `ErrInvalid`; [raw RED](q36-view-policy-red.txt) | Engine passes the strategy resource; direct Advance reads the immutable saved view without LRU touch; final BFF-wide race log below passes. |

The old/new compatibility checkpoint was run **before** replacing the old
personnel Redis code. Later wrapper-only tests assert the frozen key, bytes,
TTL, eviction, Session isolation and CAS; they do not misrepresent the
post-extraction wrapper as a second independent historical implementation.

## Environment and checks

Isolated local PostgreSQL 18.6 databases `weaveos_v015_q36` and
`weaveos_v015_q36_archive` were migrated through this branch's hot6/cold3
migrations and given the reviewed runtime/cold roles. A separate disposable
Redis container on loopback port 65433 supplied DB 15. No production or
shared database was used; Redis test keys use unique generations and cleanup.

- [Full BFF race output](q36-bff-full-race.txt):
  `go test -race -p 1 -count=1 -timeout 10m ./...` with the isolated PG,
  archive and Redis URLs, **exit 0**. Personnel package passed in 80.000s;
  querycontext and all other BFF packages passed. This includes existing
  personnel Q36 scale regression; no new benchmark matrix was added.
- [Focused personnel/querycontext race output](q36-full-race.txt): an earlier
  complete package run passed before the resource-aware Policy.Forward fix;
  the final BFF-wide run above includes that fix and supersedes this result.
- [Named HTTP/transaction regression](q36-http-regression.txt):
  `go test -race ... -run` for real Session/CSRF member search, changed
  criteria and concurrent RR, revision lock order, Redis compatibility,
  deferred COMMIT failure and lost COMMIT acknowledgement, **exit 0**.
- [Neutral RR fault regression](q36-fault-regression.txt): two new tests use
  real PostgreSQL Repeatable Read reads and real Redis. A transaction wrapper
  injects failure before PG COMMIT or returns an ambiguous acknowledgement
  after actual PG COMMIT. Both `Execute` and `Receipt.Commit` return the error
  without Redis Create/Advance. A concurrent Redis revision CAS winner after
  verified RR is tolerated on both paths; a different Redis failure after
  COMMIT is returned on both paths. The fault is at the transaction boundary,
  not an induced network failure. `go test -race ./internal/querycontext
  -run 'TestRealPGRR(CommitFailureNeverPublishesRedis|ConcurrentCASLossAndOtherRedisError)$'
  -count=1 -v` exited **0** against the isolated services. This was
  **regression GREEN on first run**: the existing extraction already had the
  required behavior, so there is no new production change or invented RED.
- `go vet ./...`, `node scripts/check-tasks.mjs`,
  `node scripts/verify-repo.mjs`, `git diff --check` and `gofmt -l` pass.
  These verify the extraction and repository structure, not the incomplete
  record product API.

ADR §11-derived DTO/document corrections accompany the extraction:
`quickSearch` is optional but explicit null is invalid, RuntimeField allows
`minute` timePrecision and finite rounding modes, and schema/view/policy
versions are control dependencies **outside** observable P. The frozen record
save history includes only record/task Save value deltas; schema Save gets
rule/version audit, with lossy old-row-value recovery explicitly unavailable.

## Remaining gate

V013 still owns the non-manager record transaction, data grants, controlled
typed-DML adapter, source registry/hooks, fence, shared OpenAPI/roles and BFF
composition. This branch does **not** attach apprecords as the second
querycontext consumer yet, does not expose quickSearch/history/runtime routes,
does not select compact A and does not close the million-row full-chain
capacity gate. No PR, main merge or deployment was performed.
