# Native history: measured trade-off, not a universal speed claim

Root-read exact head87cb6e1dcd9a6ed806882f2d104b7063b1295fef, 2026-10-06 05:53 UTC.
https://github.com/Hubujiu/WeaveOS/actions/runs/37420609449/job/112128968700

Real Flowable8/PG18.6, isolated internal network, synthetic actors. RootNativeHistoryCostTest: 1 passed, zero failure/error/skip, strict report identity checked. Frozen behavior suites8+8+18+8+36+5 also passed in this job.

## Method and observations

One real process at zero and100 return cycles; each cycle actually returns then completes both first-node tasks. Native activity rows10/810, former duplicate visit rows2/202 reconstructed only in the comparison fixture from actual task mappings. Each query has20 warmups and100 measured calls, old JDBC COUNT first and native public API second. DriverManagerDataSource is unpooled: connection/engine/JIT overhead is included; order and GC can bias timings. This is not a production load test or a per-request memory profiler.

| Return cycles | Old JDBC median/p95 ms | Native API median/p95 ms | Existing native relation bytes | Additional duplicate relation bytes |
|---|---|---|---|---|
|0|7.432 /12.842|7.736 /10.399|98,304|40,960|
|100|4.542 /5.788|5.824 /7.372|516,096|98,304|

The native API was slower end-to-end in the100-cycle sample: +1.282ms median. Do not claim all queries became faster. The independently executed SQL EXPLAIN at100 cycles showed native index scan+LIMIT1, one row/two buffer hits/0.047ms execution; old COUNT index-only scan consumed101 rows,56 buffer hits and0.096ms. Both used existing indexes; no new index or custom history store was introduced. Exact JSON plans and measurements remain in ci-test-step.txt (raw cost-step excerpt, not the entire job).

Heap observations (bytes): zero cycles71,990,840→61,558,784;100 cycles98,774,976→79,984,672. Each window contained one GC. These numbers combine both strategies and JVM activity; they are not allocation-per-query, isolated memory savings, or peak RSS. The duplicate-relation bytes are measured PostgreSQL storage including its indexes, not RAM.

## Complexity and choice

Let H be native activity rows, V be duplicate visits, K be repeated activations of the requested node. With the observed proc/node B-tree, indexed native existence lookup costs approximately O(log H + examined matches), with LIMIT1 and O(1) returned application objects; an MVCC/visibility or type-filter scan can still examine more tuples, so worst-case O(H) is not excluded. Original COUNT consumes O(log V + K) matching entries and O(1) result memory. Both may use a sequential plan on other sizes/statistics.

Native history already exists for the approved product semantics. Removing duplicate visits avoids O(V) additional persisted rows/two indexes and one redundant INSERT/index maintenance per activation (typical O(log V) index work), plus duplicated truth/reconciliation. It does not reduce the required native history O(H). Mapping/receipts/fences remain. The choice is correctness and maintenance plus bounded existence work, not a fabricated globally optimal algorithm.

No production rows, old schemas, or logs were deleted. Future task/instance lifecycle and product workflow deletion remain separate unfinished work.
