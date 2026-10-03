# V030-015 compact A and bounded digest cache: isolated experiment

Status: **candidate measured; not selected for production or complete-chain
acceptance**. The existing full-projection A remains the frozen design until
the lead reviews this evidence and every required writer invariant is proved.
Test-only code is in `services/bff/internal/appquery/compact_candidate_test.go`,
`compact_cache_test.go`, and `compact_scale_test.go`. The target RED logs are
[compact oracle](compact-candidate-red.txt) and
[cache](compact-cache-red.txt). Local PostgreSQL 18.6 GREEN and raw costs are
[100k same](compact-scale-100k-same.txt),
[100k distinct](compact-scale-100k-distinct.txt),
[1m same](compact-scale-1m-same.txt), and
[1m distinct](compact-scale-1m-distinct.txt).

## Conditional equivalence argument

The independently assembled complete oracle P hashes **each authorized,
matching row in query order** with ID, createdBy/createdAt, updatedAt,
recordVersion, numeric/text values, per-row field-mask keys and the actual
visible reference `(sourceId,label,deleted)`; it also includes COUNT. The
candidate orders the **same SQL-authorized/filter-matched rows** by the same
stable sort and hashes fixed-width `(id[16],recordVersion[8],own-mask[1])`.
It binds the canonical effective field mask to the signature and separately
hashes sorted distinct `(sourceId,label,deleted)` only for references visible
in those matching rows, in the **same RR snapshot**. The distinct source set
can be shared among rows because a changed row-to-source assignment must
increment that row's recordVersion. A source revision is a reason to
recompute; it is not automatically a relevant result change.

If two observations have the same compact row sequence/count, effective mask
and visible-source digest, then P is equal **only if** every business/system
value mutation of an existing row increments recordVersion, createdBy and
createdAt are immutable, every sort/filter field mutation obeys that rule,
all source display/deleted changes are captured by the authoritative registry
query, and schema transformations advance schemaVersion. A schemaVersion
change invokes the **complete** P oracle under the saved criteria; it does
not by itself force a user-visible refresh. An invalid old criterion after
field removal still follows the frozen Q36 change/error rule. Policy changes
require live authorization first and either a proven canonical effective-mask
comparison or complete P fallback. The compact hash does not replace the
full oracle until these conditions are verified for all paths.

The real PG equivalence test covered no-op, unmatching row edit, unmatching
source rename, visible text edit, hidden-field edit with visible version/time,
numeric sort reorder, visible source rename and deletion, effective field-mask
change, membership entry, row reference reassignment, own-only reference
masking, and rename of a now masked source. Each expected relevant/irrelevant
answer matched the full JSON oracle. A deliberate bad writer changed a
visible business value **without** incrementing recordVersion or updatedAt:
P changed and compact did not. This is a demonstrated false negative if the
writer invariant is missing, not a passing proof of the current product.

Current V015 `apprecords.Writer` requires its TypedDML port to return a
version increment for every nonempty edit; the reviewed runtime adapter must
protect system columns and maintain the server timestamp, but
V013 schema conversion and option remapping can update typed values without
per-row recordVersion changes. The lead's schemaVersion fallback is therefore
essential. The reviewed runtime typed-DML adapter, all future task Save
writers, existing/migrated typed tables, V013 schema conversion entrypoints,
source hooks and any owner/admin paths must be inventoried and exercised as
restricted `auth_app` before compact A can be adopted. The current fixture
has no production registry/member-department integration, so its 100-source
label query is only a representative cost and correctness sample.

## Cost and cache behavior

The fixture has N typed PG rows, one numeric sort/filter field, text/secret
fields, one reference to 100 stable source IDs, an all-row read plus an
own-only secret field, and indexes for `(created_at DESC,id DESC)`, numeric,
and reference ID. Tables were separately seeded and analyzed. The scale
fixture was static with no competing writer, so its two PG reads did not use
an explicit RR transaction; the equivalence test did, and any production
strategy must. All timings are one local warm run; C=200 is **sequential** hypothetical contexts, not
200 concurrent users or a p95 service SLA. Each distinct criterion is
`number >= -i` for i=1..200; all match N rows, so this is a worst case for
cache reuse and row volume, not every possible query plan. The cache holds
only digest, count and revision tuple, with 32-entry bound and 5-second TTL.
Its key in the fixture includes actor, actual resource, canonical criterion,
effective field mask and revision tuple. No records or Session contexts are
cached. A separate 200-goroutine test observed one compute for a shared key;
actor, mask, criterion and revision changes each forced a miss.

| N | 200 same key: misses / total | 200 distinct keys: misses / total | Distinct call p50 / p95 / p99 | One complete P |
| ---: | ---: | ---: | ---: | ---: |
| 100k | 1 / 92.3 ms | 200 / 24.13 s | 121 / 132 / 140 ms | 594 ms |
| 1m | 1 / 956 ms | 200 / **4m 02s** | 1.186 / 1.401 / 1.605 s | 6.47 s |

At N=1m the cold compact call split into 543 ms ordered row stream and
412 ms distinct visible-source query. The row columns contribute exactly
25 MB of binary **column payload** per context plus about 3.3 KB of source
values; PostgreSQL protocol framing, socket buffers, query executor memory
and indexes add more. The corresponding complete JSONB stream returned
301.68 MB through Go. At 1m/200 distinct the test-only Go process allocated
17.60 GB cumulatively; its final heap was 1.62 MB. Relation plus indexes
occupied 301.31 MB. The 100k full stream returned 30.07 MB versus 2.50 MB
compact column payload. These figures do not include deployed network hops,
Redis, Session/policy loading, reference fanout across member departments,
write/WAL/lock contention or browser/API latency.

The identical-key case demonstrates why digest single-flight can reduce
*duplicate* work after a shared revision, provided each Session independently
revalidates current permission and its own Q36 queryVersion. The 200-distinct
case shows the cache cannot solve arbitrary one-million-row workload cost:
4m02s sequential and 200 PG scans still occur. TTL expiry, a different
actor's own range, a different field mask, criteria or any observed revision
requires another compute. Cache publication must occur only after the RR
observation commits and must use bounded CAS/lifecycle semantics from the
shared Q36 engine. A cache hit cannot substitute for live Session, current
menu/data grants, current schema/source availability or per-Session context.

## Decision and remaining gates

No code path now swaps `FingerprintRows` for compact A or installs this cache
in the application. To consider adoption, the lead must approve the precise
P/system/reference boundary and review a proof/integration test for **every**
writer that can mutate typed values, system columns, schema, option labels,
registry labels, tombstones and member-department display. Then run the
full real-role, RR/context/Session/API and 10k/100k/1m × 1/20/200 matrix,
including 200 different filters, own/all masks, source fanout, unrelated
writes and schema/policy changes. Broad full A's earlier >600-second failure
remains an open capacity gate until a chosen strategy passes complete-chain
acceptance. No timeout was enlarged in these experiments.
