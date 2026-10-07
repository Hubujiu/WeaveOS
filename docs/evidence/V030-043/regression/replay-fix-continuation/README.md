# Authorized continuation after catalog scale failure

Frozen source44b2058ac2ff4de107e2ae9a0f20cc4cbb230fa1, execution head1d374c8a809bceca8e397e8cceadc01154ba11cd (preceding evidence-only commit). All implementation/test/dependency hashes unchanged. Original isolated hot/archive PostgreSQL and Redis reused. No fixture, schema, timeout, pool or planner-setting alteration.

Root explicitly authorized the four packages that exclude the failing scale test, plus vet/build. Exact sequential commands/timestamps/cases/source/artifact hashes in run.json. Each race package ran `go test -race -p 1 -count=1 -json -failfast <package>` with default tags. No failure or skip, every stderr empty, no race diagnostic.

| Run | Exit | Top-level PASS | Subcase PASS |
| --- | --- | --- | --- |
| race-apprecordhttp | 0 | 0 | 0 |
| race-appstructure | 0 | 118 | 26 |
| race-applications | 0 | 29 | 12 |
| race-bff | 0 | 66 | 42 |
| vet | 0 | 0 | 0 |
| build | 0 | 0 | 0 |

apprecordhttp has no test files and compiles successfully. Four-package totals213top-levelPASS/80subcasePASS. Vet covers apprecordservice,apprecordhttp,appstructure,applications,cmd/bff. Build compiles cmd/bff to a retained task-private binary; only binary SHA256 submitted, no service deployed or launched by that build command. Go1.27.1, CGO enabled, verified cached official modules, GOPROXY=off/GOSUMDB=sum.golang.org.

Overall is NOT all-green: apprecordservice full race stopped at the catalog scale assertion in ../replay-fix/; that test was not rerun. Replay-specific20top/2sub GREEN is also in that preceding evidence. Formal workflow engine combination acceptance remains Root-owned and not run, as do other packages, frontend/browser/product/final CI. No implementation/test changes, PR, merge or deployment. Raw JSONL/stdout/stderr/exit preserved byte-for-byte; full private binary path retained only in task-private run metadata.
