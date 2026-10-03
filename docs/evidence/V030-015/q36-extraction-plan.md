# V030-015 · Q36 neutral query-context extraction plan

Status: **frozen and released for the exact extraction scope** by V015 ADR
§12, read back 2026-10-03 11:49:12 UTC. Implementation and real regression
evidence are recorded separately; this plan is not product API acceptance.
Reviewed the fixed V015 worktree at `1633361e69060011e25d780f39c0ec68dbab3512`
against the live `personnel/query_context.go`, `query_engine.go`,
`query_write_guard.go`, projection and HTTP/PG/Redis tests on 2026-10-03.
The lead assigned those three personnel source files, their corresponding
Q36 tests and a new `services/bff/internal/querycontext/` package to V015.
Other personnel files, especially source hooks, remain V013-owned. This plan
does not authorize a runtime record route, history route, quick-search route,
new benchmark matrix or shared migration edit.

## Current behavior to preserve byte-for-byte or semantically

1. Redis personnel prefix is
   `ems:personnel:query:<generation>:v1:{sha256(lowercase sessionRef)}:`.
   A random 32-byte base64url token identifies each context. The LRU is one
   ZSET per Session, shared by member and event contexts, capped at **20**;
   successful Create/Load/Advance refreshes its **30-minute idle TTL** and
   monotonic access score. The hash contains only `data` and `revision`.
   `data` has the current JSON field names/order and protocolVersion `1`;
   `revision` is the separate JSON encoding of
   `{People,Configuration,Activity}`. Existing unexpired tokens must load
   and advance after extraction without a Redis migration.
2. The personnel validator keeps its closed criteria vocabulary, 64-KiB
   maximum, finite 53-bit total, 64-character lowercase SHA-256 fingerprint,
   valid member/event revision shapes, event frozen time bounds and accepted
   filters/sorts. The generic store may enforce framing/size and opaque ID
   syntax, but the personnel adapter retains `CompileFilter`, action names,
   relative event range freezing and view-specific validation. It cannot
   weaken or replace these with a permissive JSON object check.
3. Query reads load the token **before** opening a PG Repeatable Read read-only
   transaction, then perform live `a.read` authorization **before** returning
   token expiry/invalidity. Saved/current criteria, revisions, old full
   projection, new full projection or page-only query, hydration, COUNT and
   authorization use that one RR snapshot. Existing personnel filter/range
   normalization and error precedence stay in its adapter. Unchanged
   revision plus same criteria reads only the requested page and saved total;
   a changed revision rehashes the complete old result. Equal digest+COUNT
   permits CAS advancement, including ABA; unequal yields `QUERY_CHANGED`.
   Different incoming criteria create a new immutable baseline only after
   the old token validates. Page beyond total remains an empty valid page.
4. Only **after PG RR COMMIT** may Redis Advance/Create occur. Advance changes
   only the revision field, with exact previous-revision and immutable
   fingerprint CAS; it never overwrites criteria, total or fingerprint.
   A concurrent Advance CAS loss is harmless after a verified RR result;
   other Redis errors fail. There is no PG/Redis atomic transaction and no
   publication before DB commit. Existing ten-second personnel read deadline
   and `COMMON_QUERY_*`/`QUERY_BUSY` HTTP mapping remain unchanged.
5. The separate Q36 write receipt first verifies a saved context in RR and
   commits that read; the RC business transaction then takes
   `personnel.lock_query_revisions()` before actor authorization, rechecks
   the verified revision and retries at most three **pre-mutation** attempts.
   Business mutation and COMMIT are executed once. Existing HTTP routes that
   require queryVersion keep doing so; direct detail/service methods that
   currently permit no version remain so. Unknown COMMIT is never retried.
   Existing catalog/department/definition reads use the same RR receipt.

## Minimal interface and ownership

Move the **single** Redis implementation and lifecycle orchestration to
`internal/querycontext`. Do not copy Lua, invent a second Redis context, or
move personnel SQL/projection/filter rules into it. Proposed signatures are
illustrative Go contracts for the freeze review, not code already present:

```go
// Neutral package; revision bytes are canonical JSON supplied and checked by
// each domain. The personnel adapter preserves its existing JSON encoding.
type Metadata struct {
    View string `json:"view"`                 // exact resource discriminator
    Criteria json.RawMessage `json:"criteria"`
    Total int64 `json:"total"`
    Fingerprint string `json:"fingerprint"`
    ProtocolVersion int `json:"protocolVersion"`
    Revision json.RawMessage `json:"-"`       // separate Redis hash field
}
type Policy interface {
    Validate(Metadata) error                  // closed, domain-specific shape
    Forward(view string, old, next json.RawMessage) bool
}
type Store struct { /* one Redis implementation, fixed namespace/generation */ }
func NewStore(redis.UniversalClient, namespace, generation string, Policy) *Store
func (s *Store) Create(context.Context, sessionRef string, Metadata) (token string, err error)
func (s *Store) Load(context.Context, sessionRef, token string) (Metadata, error)
func (s *Store) Advance(context.Context, sessionRef, token, fingerprint string,
    previous, next json.RawMessage) error

type Page struct { Number, Size int }
type Observation[T any] struct { Items T; Total int64; Fingerprint string }
type Strategy[T any] interface {
    Resource() string
    OpenRead(context.Context) (pgx.Tx, error) // current Session/ACL, RR read-only
    Prepare(context.Context, pgx.Tx, saved, incoming json.RawMessage) (
        old, current json.RawMessage, err error)
    Revisions(context.Context, pgx.Tx) (json.RawMessage, error)
    Observe(context.Context, pgx.Tx, criteria json.RawMessage, page Page) (Observation[T], error)
    Page(context.Context, pgx.Tx, criteria json.RawMessage, page Page) (T, error)
}
// Execute and ValidateSavedRead use the same Store and Strategy. The latter
// returns an RR transaction plus a Receipt whose Commit performs Redis CAS
// only after the caller finishes all other reads in that same snapshot.
```

`Prepare` owns the domain's saved-criteria validation, current canonical
criteria, live filter/sort permission checks and error precedence. Personnel
can preserve its current incoming normalization before old projection;
records can validate the saved old criterion first, as the V015 ADR requires.
`Observe` always returns the complete authorized result digest/count plus
the requested page, while `Page` is the small unchanged-revision path.
The strategy hydrates its own page **inside** the passed RR transaction.
Neither Store nor Execute sees business rows, filter ASTs, SQL, grants or
physical table names. `OpenRead` must fail closed on current Session/rights;
the engine still calls it when a token load failed, preserving auth precedence.

Personnel keeps its exported `QueryContext`, `QueryRevisions`,
`NewQueryContextStore`, `SearchMembers`, `SearchEvents`,
`BeginQueryWrite` and error identity as compatibility wrappers/aliases.
`Application.Queries` and HTTP composition need no type or route change.
The personnel adapter uses `namespace="personnel"`, exactly the current
key format and JSON revision struct. `query_projection.go` stays in
personnel; its member/event SQL and fingerprint framing do not move. Its
`queryReadReceipt` may delegate to neutral `ValidateSavedRead`, but
`BeginQueryWrite` retains the actual personnel RC lock/authorization order.
`commitBusiness` and draft cleanup stay in personnel. V015 apprecords will
later use `namespace="applications"`, a resource key binding real
app/table/view IDs, and an app-specific validator/strategy **on the same
neutral Store/Execute/Receipt**; its non-manager transaction/operation port
remains V013-owned. No app implementation is part of this extraction plan.

## Record revision vector proposed for later adapter

The saved record context must bind the canonical criteria and resource key
plus a monotonic vector covering actual table `dataRevision`,
`dependencyRevision`, `schemaVersion`, form `viewVersion`, app
`policyRevision`, and every authoritative reference registry/member-
department source revision that can affect P. V013's source-hook naming and
counter coverage are not yet frozen; missing counters fail unavailable,
never compare as zero or unchanged. Resource and actor/Session are checked
separately on every read. A revision difference triggers a fresh full P
observation in the same RR; it does **not** itself produce a user-visible
refresh. Saved old filter/sort loss of full read coverage yields 403; an
authorized but tombstoned saved field yields `QUERY_CHANGED`. Compact A
remains isolated and unselected; the initial record adapter uses full P.

## Frozen implementation sequence

1. Capture a real Redis pre-extraction compatibility fixture: prefix,
   serialized data/revision, 20+1 eviction, Session isolation, TTL and CAS.
   Add failing neutral-store/parity tests first and retain RED evidence.
2. Move Lua and generic storage into `querycontext`, keeping the personnel
   wrapper and wire bytes. Prove a token created by the old implementation
   can Load/Advance with the new one and the reverse for the freeze test.
3. Add failing neutral lifecycle tests for RR old/new/page paths and receipt
   publication order. Extract the one lifecycle from `query_engine.go` and
   `query_write_guard.go`; leave personnel domain adapters in those files.
   Preserve the personnel route signatures and error mapping.
4. Run real PG+Redis Q36 regression: context, projection, query HTTP,
   changed-criteria RR, event references/archive, frozen filter, Unicode,
   write guard, revision lock order, draft cleanup and lost-COMMIT tests.
   `go test -race ./internal/personnel ./internal/querycontext` plus vet and
   the actual HTTP tests are the minimum. Do not broaden the scale matrix.
5. Only after V013 shared fields/grants/source hooks/transaction ports and
   the Notion freeze land, attach apprecords as a second **consumer** and
   test real role, Session, ACL, same-RR P, Redis CAS, refresh and writer
   invariants. Full product/browser and capacity gates remain separate.

No personnel HTTP behavior, Redis key, limit, deadline, scope, source hook,
business write or operation receipt is proposed for change in this stage.
