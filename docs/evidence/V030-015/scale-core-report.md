# V030-015 isolated PostgreSQL query-core capacity probe

Status: **measured core stage; the broad 1m × 200 case failed the unchanged
10-minute test deadline**. Raw outputs: [C=1 and query plans](scale-core-plans.txt),
[whole matrix timeout](scale-core-timeout.txt),
[isolated broad 1m × 200 timeout](scale-core-1m200.txt),
[10% 1m × 200](scale-core-1m200-ten-percent.txt), and
[0.1% 1m × 200](scale-core-1m200-point-one-percent.txt).
Harness: `services/bff/internal/appquery/scale_test.go`.

This used local isolated PostgreSQL 18.6, real typed columns, indexes and
`ANALYZE`. Every N is a newly seeded TEMP table with stable UUID, an immutable
`created_by` alternating between two actors, numeric, time and text. The
`created_at,id`, `created_by,created_at,id`, and numeric field indexes occupied
the relation bytes below together with the table. Projection uses ordered
`jsonb_build_array` and `FingerprintRows`; it hashes one PG row at a time and
does not materialize the full table in Go. C is **sequential re-observation of
distinct hypothetical contexts after a relevant change**, not concurrent
connections or live Redis contexts. All timings are warm local samples; the
200-call p95/p99 below are distribution among serial calls, not a service
latency promise.

## Full projection cost

| Rows N | C=1 | C=20 | C=200 | Bytes streamed per context | Relation + index bytes |
| ---: | ---: | ---: | ---: | ---: | ---: |
| 10,000 | 23.7–32.0 ms | 416 ms | 4.64 s | 1.43 MB | 3.08 MB |
| 100,000 | 268–318 ms | 5.01 s | 62.9 s | 14.38 MB | 29.80 MB |
| 1,000,000 | 3.24–3.56 s | 45.3 s | **timeout >600 s** | 144.78 MB | 296.93 MB |

The first three timing columns combine the early whole-matrix run and the
later C=1 instrumented rerun; hardware/cache state varied. Both whole-matrix
and isolated broad million-row C=200 runs hit Go's **default 10-minute test
deadline before emitting a successful REHASH result**. We did not raise the
deadline to report a pass. At one million rows, the C=20 run streamed 20m
records and allocated about 3.94 GB cumulatively; the later C=1 run allocated
about 197 MB while its final Go heap remained about 1.6 MB. The process
maximum resident memory was about 27–29 MiB in the later capped/filtered
runs; this excludes PostgreSQL process memory, caches and all live service
components. Network traffic in a deployed configuration would be at least the
streamed JSONB bytes if PG and BFF are on separate hosts.

Selectivity changes the number of projection rows but still requires an
authorized SQL predicate and matching indexes:

| Million-row source, C=200 | Matches/context | Total rehash | Sequential call p50 / p95 / p99 | Bytes/context |
| --- | ---: | ---: | --- | ---: |
| Numeric `<100` (10%) | 100,000 | 63.6 s | 297 / 459 / 538 ms | 14.38 MB |
| Numeric `<1` (0.1%) | 1,000 | 0.531 s | 2.52 / 3.47 / 4.78 ms | 0.143 MB |
| Unfiltered | 1,000,000 | timeout >600 s | no successful distribution | 144.78 MB from C=1 |

The predicate distribution was uniform by fixture design; these figures do
not extrapolate to uneven tenant data or reference predicates. Under frozen A,
even an unrelated write that bumps the same table revision can require
`O(C × M)` rehash rows across C contexts, where M is the authorized matching
set. An unchanged projection avoids a **user-visible refresh**, while the
rehash still incurs CPU/PG I/O/network cost. That cost is **not** bounded by
the 20/100-row page size. This evidence is for the lead's capacity decision
and does not silently substitute another algorithm for frozen A.

## Count, pages and plans

The C=1 run measured the following one-off local queries, with no live
Session, policy or reference join overhead:

| N | COUNT all | COUNT own (half) | COUNT numeric 1% | Page 20 shallow | Page 20 OFFSET N/2 | Page 100 OFFSET N/2 |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 10k | 1.59 ms | 1.24 ms | 0.79 ms | 0.53 ms | 1.97 ms | 1.68 ms |
| 100k | 14.24 ms | 12.00 ms | 1.05 ms | 0.66 ms | 18.0 ms | 14.6 ms |
| 1m | 153.7 ms | 97.6 ms | 6.72 ms | 0.84 ms | 200.5 ms | 178.0 ms |

At N=1m, compiled one-leaf and 20-leaf numeric predicates counted 10k and
20k rows in 6.94 and 9.00 ms. `EXPLAIN (ANALYZE,BUFFERS)` for page size 100,
OFFSET 500k showed an Index Only Scan **visiting 500,100 rows**, with 500,100
heap fetches, 10,603 local blocks read and 127.2 ms executor time. The full
ordered projection used an Index Scan, read 20,537 local blocks and took
1,314 ms executor time; BFF streaming/hashing raised the observed end-to-end
core call to 3.24 s. COUNT all used Aggregate, read 16,394 local blocks and
took 148.1 ms executor time. These fixture plans had zero temporary spill.
`OFFSET` is `O(offset + pageSize)` even with a sort index; arbitrary page
numbers remain supported, without a constant-time deep-page claim.

## Complete-chain acceptance still needed

This fixture **does not include** shared Q36 Redis context/CAS, Session,
restricted `auth_app`, persisted data grants, RR revision reads, source
registry/reference display, operation ledger, BFF/API, browser, concurrent
clients or write/lock/WAL effects. It measures neither unrelated-write
fast-path success nor stale-context user refresh. After V013-owned shared
ports land, repeat N=10k/100k/1m × C=1/20/200 through the actual chain,
vary own/all masks, selectivity, 0/1/20 conditions, page size 20/100,
shallow/deep pages, reference fanout and relevant/unrelated writes. Capture
COUNT/page/rehash, repeated p50/p95/p99, `EXPLAIN (ANALYZE,BUFFERS)`, temp
spill, roles, WAL, PG/BFF memory and lock wait separately. The broad
million-row × 200 context timeout remains an explicit capacity issue for
lead review before product acceptance.
