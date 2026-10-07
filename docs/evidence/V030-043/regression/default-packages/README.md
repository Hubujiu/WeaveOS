# Default-package broader regression at dbdb67f3

Exact source: `dbdb67f3f444d6fbda34e121196870b3501534fe`, implementation `fa7d05b5c844fc11f78bab161022bc72ddc26843`. Only executor metadata-description corrections were uncommitted during execution; no production/test/dependency/fixture changes. These results precede Root's additional replay resource-boundary test at32f4b88. They do not mean the later head is fully green.

Read docs/testing.md, CI Go job and actual package fixture requirements first. Existing task-only PostgreSQL18.6/Redis8.2.10 were reused; the required independent synthetic archive database was created inside the same task-specific PostgreSQL container, six original archive migrations applied and original cold-roles.sql applied, each exit0. No new container, network policy, real credential or published port.

Each package ran sequentially with `go test -race -p 1 -count=1 -json -failfast <package>`. -failfast implements Root's stop-at-first-failure instruction without removing any test selection; no failure occurred so each default package completed in full.

| Package | Exit | Top-level PASS | Subcase PASS | FAIL/SKIP |
| --- | --- | --- | --- | --- |
| internal/apprecordservice | 0 | 184 | 95 | 0/0 |
| internal/apprecordhttp | 0 | 0 | 0 | no test files; package compiles |
| internal/appstructure | 0 | 118 | 26 | 0/0 |
| internal/applications | 0 | 29 | 12 | 0/0 |
| cmd/bff | 0 | 66 | 42 | 0/0 |

Totals397top-levelPASS/175subcasePASS,0FAIL/0SKIP. Exact case names and original JSONL are retained. Go1.27.1, CGO enabled, official-verified dependencies already cached, GOPROXY=off/GOSUMDB=sum.golang.org. Every Go stderr empty; no race diagnostic.

`go vet ./internal/apprecordservice ./internal/apprecordhttp ./internal/appstructure ./internal/applications ./cmd/bff`: exit0.
`go build -o <task-private-bff-binary> ./cmd/bff`: exit0. Binary retained privately; SHA256 in bff.sha256. No service launched by this build command.

Not run: other BFF packages, workflowrpc_integration Java interoperability, workflowruntime_integration actual dual-process scenarios, frontend/browser/product suites or final CI. Default-tag package tests include existing real HTTPS/host/main process coverage; they do not replace excluded tagged engine acceptance.

Raw JSONL/stdout/stderr/exits are byte-preserved after credential/DSN/private-path review; public metadata describes the private storage endpoints, and full original metadata remains outside Git. Original/submitted hashes and per-case counts are in run.json. All frozen file hashes unchanged.

Metadata correction: allowedPaths already contains record_http_decode.go at d30da6bc, so Root's current contract required no path edit. The executor's prior obsolete statement was corrected in its task/first-GREEN prose only. Root contracts/raw historical logs are unchanged.
