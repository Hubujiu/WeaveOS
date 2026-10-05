# V030-031 codec evidence

Root authored the frozen contract, 15 behavior tests, eight Python-generated normative byte/hash vectors and four benchmarks. GPT-6.1-sol medium implemented only execution_codec.go. The cloud execution agent preserved and ran the original RED, reviewed the implementation, and ran GREEN/regressions/benchmarks; main-conversation Root source review and acceptance remain pending.

## RED and immutable oracle

Original Root HEAD5a599997 compiled successfully (2.722s), then all15 named top-level codec cases ran:5pass/10stub behavior fail/0skip, exit1 (0.373s). Original command/output/exit/hash/source snapshots live in red/. No assertions, generator or vector bytes changed. Test-source formatting equals exact gofmt(original); original/formatted hashes and byte comparison are in green/test-format-equivalence.json.

## GREEN and fixture

Go1.27.1 linux/amd64 with existing SDK/module cache, GOPROXY=off. Dedicated loopback-only PG18.6/Redis8.2.10 fixture: all16 existing migrations, then existing unmodified infra/runtime/roles.sql. First formal regression attempted an incomplete fixture (auth_app absent):21fail. Its original logs remain recorded as fixture-incomplete; after applying existing role setup, all21 formal Ledger/fence tests pass without skips. This was an environment preparation failure, not codec RED.

Cloud agent runs: new codecs15pass/0skip (2.074s); full flowcommands61pass/0skip, including all46 old realPG/fault/protocol cases (2.339s); formal Ledger/fence21pass/0skip (4.726s); vet3.060s and build0.847s, exit0. Raw GoJSON/stderr, commands/timestamps/wall times and environment are under green/. Governance/foundation tests, verify-repo and check-tasks passed (2.692s); services/bff gofmt and git diff --check passed. Protected existing Command/Receipt/PlanReceipt/canonical/Ledger and dependency/role source files remain byte-exact; no migration or dependency edits.

## Actual benchmarks

Command: go test -run '^$' -bench '^BenchmarkRootExecution' -benchmem -benchtime=1s -count=5 ./internal/flowcommands. No race instrumentation; actual total wall time30.912s. Five observed samples per case, median ns/op and observed byte/allocation ranges:

| Case | Median ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| PayloadSmall | 842.8 | 432–432 | 3–3 |
| PayloadMax | 552598 | 489378–489379 | 403–403 |
| ResultSmall | 2589 | 1016–1016 | 13–13 |
| ResultMax | 43607 | 26104–26104 | 211–211 |

Raw results, CPU/OS/toolchain and all samples are preserved. Max payload is208550 wire bytes:100 approver-map entries with50 actors each plus100 route-map entries. It does not assert a legal200-node business graph. Max result is50 tasks with200-byte engine IDs. These measurements describe this cloud machine and current implementation; no best-algorithm or production throughput claim.

## Limits and next action

Pure bounded wire codecs only. Hash checks bind a body to the existing receipt, not service authentication. No-effect unchanged results must not overwrite current projections. No HTTP, auth, DB business wiring, Java behavior, main merge or deployment is included. Root source review and exact-head CI remain required before ready/acceptance.
