# V030-015 · records consumer wiring at the V013 shared port boundary

Status: **integration plan only; record HTTP and capacity acceptance are open**.
Based on the frozen [V015 ADR §11–12](https://app.notion.com/p/3ee2f5a9e6488163a144fba5f55e3c14), the [single Q36 extraction](q36-extraction-evidence.md) at `0c537bb43cdafae405c515909c1bcebfa177a0c9`, and the read-only [V013 shared-port commit](https://github.com/Hubujiu/WeaveOS/commit/46cf01b0d7b79cebe4d50dca2667083ab9b850d9) ([owner guide](https://github.com/Hubujiu/WeaveOS/blob/46cf01b0d7b79cebe4d50dca2667083ab9b850d9/docs/evidence/V030-013/integration-guide.md)). V013's branch is stacked on a newer PR26 base than this V015 worktree. Coordinate one integration base with the lead before changing shared files. No record consumer source is attached in this checkpoint.

## Smallest attachment

1. In V015-owned `appquery` or a thin V015-owned application query service,
   implement `querycontext.Strategy[RecordPageItems]`. The actual resource key
   binds app, table and view IDs. Use **the existing**
   `querycontext.Store/Execute/ValidateSavedRead`, with namespace
   `applications`, SessionRef from the trusted principal, and a closed record
   metadata/revision policy. There is no second Redis Lua store, lifecycle or
   personnel adapter. V013 owns BFF route composition and shared DTO/error
   files; V015 supplies the service call and precise contract delta.
2. `OpenRead` authenticates the live Session and checks the real form/menu,
   active data.read and source availability before returning a read-only PG
   Repeatable Read transaction. `Prepare` validates the saved **old** record
   criteria and full field read coverage before incoming criteria. Compile
   filters/sorts with `appquery.CompileSearch` (and frozen quickSearch when
   implemented), then `appaccess.CoversRead`; inaccessible saved fields yield
   403 and tombstoned saved fields yield `QUERY_CHANGED` for an otherwise
   authorized actor. Never treat an expired token as authority.
3. `Revisions` reads the actual table data/dependency, app policy, schema,
   view and required reference-source revisions **inside the same RR**. A
   missing source hook/counter is unavailable, not revision zero. `Observe`
   applies `appquery.CompileAccess` in SQL **before** filter, COUNT and page;
   streams complete ordered P through `FingerprintRows`, with ID, order,
   recordVersion, system times, readable field values and readable reference
   `(id,label,deleted)` display. Metadata control versions stay outside P.
   `Page` runs only the requested indexed/guarded OFFSET page with the same
   authorization, masking and in-RR reference hydration. For no reference
   condition, resolve only distinct page IDs after pagination; a reference
   predicate joins the registry/edge facts before COUNT/LIMIT.
4. Convert the result to the frozen `RecordPage` including the returned
   queryVersion and actual control versions. An explicit refresh has no old
   token and requests page 1. Unchanged old token + revision reads only its
   page; changed revision reobserves full old P; changed criteria validates
   old P then observes new P. This is selected full-projection A. The
   compact/cache experiment remains unselected, and deep OFFSET is not
   described as constant time.
5. For a list-origin record mutation, validate the same token through
   `ValidateSavedRead` and commit its read receipt before business mutation.
   Then call V013 `BeginRecordWrite` with Session principal, actual form,
   original operation key/fingerprint, timeouts, source lock callback and
   trusted policy callback. Use `Replay` before `Claim`/stale-version checks.
   The write still rechecks current policy and revision under its own RC
   lock/gate before mutation; an RR receipt is an interaction guard, never a
   grant. Preserve the original key after an ambiguous business COMMIT and
   use the operation read route for recovery. No automatic new-key retry.
6. Adapt V015 `apprecords.TypedDML` to V013
   `appstructure.RecordDML{Insert,LockHeader,UpdateCAS}` by exact neutral
   struct conversions, and inject `RecordGate`, `RecordFence`, `RecordAudit`.
   Normalize values with current `appfields.NormalizeValue`, validate new
   active references, resolve complete V013 `RecordContext.Grants` into
   `appaccess.Policy`, and perform authorization/fence in the same
   caller-owned transaction. `Writer` does not begin/commit or issue direct
   business-table DML. A draft submit checks exact owner/schema/base/draft
   version and consumes the draft in the same record/audit/operation commit.

## Exact V013 ports now present; remaining shared ownership

V013 `46cf01b` supplies `applications.BeginRecordWrite`, `RecordWriteOptions`
(`SourceGuard`, `Authorize`, operation fingerprint and limits), `RecordContext`
(live actor, actual table/schema, fields and full grant tuples),
`RecordWrite.{Replay,Claim,Complete,Commit,Rollback,Tx}`, finite controlled
`appstructure.RecordDML`, `RecordGate`, `RecordFence`, minimum `RecordAudit`,
hot8 grants/drafts/fences/source registry and source revision hooks. These
remove the earlier hard blocker for a restricted-role transaction adapter.

The shared owner still controls record/search/draft/runtime/history HTTP
composition and OpenAPI/errors, restricted old/new history storage, and any
source reader used to hydrate display in RR. The current V015 exclusive core
has no quickSearch SQL or record Strategy yet. V013's guide explicitly says
record HTTP and save-history are unimplemented. The lead must coordinate the
shared integration base and owner edits; this plan does not claim those routes
exist. No Q36 or product capacity benchmark is added by this plan.

## First integration proof after base coordination

Add independent tests first for restricted `auth_app` typed create/edit,
all/own complete-tuple COUNT and field masking, live Session/CSRF,
reference tombstone and multi-department display, pending fence, draft
consume and same-transaction audit/operation, lost-COMMIT original-key
recovery, Redis token/CAS and changed/irrelevant P, and page-only versus
full rehash. Then run actual record HTTP through PostgreSQL and Redis and the
frozen million-row/200-context full-chain matrix. The existing core PG and
Q36 tests do not certify this attachment.
