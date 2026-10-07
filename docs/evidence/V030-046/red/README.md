# V030-046 two original pre-implementation runs

Root requests byte-preserved original JSONL and run.json for independent assertion review. Executor changed no production/test/fixture. Each attempt retains its exact original test and declaration-only stub snapshot; all copies match private original bytes. Original stdout/stderr/exits remain private, with exit codes and raw hashes recorded in each unchanged run.json. No credential-bearing state file copied. TDD:N/A for this evidence-only commit.

| Attempt | Exact source | Exit | Actual result |
| --- | --- | --- | --- |
| initial-mixed | 3f3b287f0a6595b4c57a41f03d6178e10051819f | 1 | 7FAIL/0PASS/0SKIP:6 valid-read declaration behavior RED;1 own-scope fixture SQL failure |
| own-fixture-fixed | 141754a07a9dac3827edc28ce0c2de502c7c6d1a | 1 | 7FAIL/0PASS/0SKIP:7 actual declaration behavior RED; no fixture/compiler/environment failure |

IMPORTANT: initial OwnScopeRejectsForeignRecord fails at line30 on fixture UPDATE with SQLSTATE23505 (grants unique constraint), before SearchRecordWorkflows. It is not behavior RED. Root repairs only fixture grant setup in141754a; second own-scope reaches line39 expecting ErrMissing and actually receives declaration ErrUnavailable. Other cases stop on valid reads returning ErrUnavailable; later revocation/change/page/session assertions remain unreached at the declaration stage. No claim that all deeper assertions have executed.

Both original commands: `go test -race -p 1 -count=1 -json -run '^TestRootWorkflowRead' ./internal/apprecordservice` from services/bff, Go1.27.1, real task-private PG18.6/Redis8.2.10 reused, original migrations1..21 applied; empty stderr/no race diagnostic. Exact SDK argv/environment/times/source hashes/case errors are in unchanged run.json. Prior run remains original, not replayed or reclassified as full RED.

| Byte-preserved artifact | Bytes | SHA256 |
| --- | --- | --- |
| initial-mixed/test.stdout.jsonl | 9704 | 772392c793a8b49a2ef7d4ca55e87ed579364a19204b678ca39f7e58f6dc3238 |
| initial-mixed/run.json | 5372 | ffc63747898e4698880f7672315ed3a4622877a324e256e2524df9fd5a61b10b |
| initial-mixed/root_workflow_read_test.go | 4790 | c96c3e756c27b9b5bee7c572e9b64e22f72dd845a6d11224ead611ab3b77e38a |
| initial-mixed/workflow_read.go | 784 | 91a4121615e672a7ee4da22283ac1d2a50351290b8c6ac97059e7c8dc9828e03 |
| own-fixture-fixed/test.stdout.jsonl | 9665 | 53ccb7146950dd9eb55aff466e3df90c15ca2966abc94ecb6cbea7d8b72286ef |
| own-fixture-fixed/run.json | 4519 | 8dd05d1b8d085fa3c6886509244885dba208978c5e2fea12f7c0169d1add68ef |
| own-fixture-fixed/root_workflow_read_test.go | 5031 | ff4490e9e868dc82c7f61cccf7e7bbe8993d3da349d75ead58141982591f182b |
| own-fixture-fixed/workflow_read.go | 784 | 91a4121615e672a7ee4da22283ac1d2a50351290b8c6ac97059e7c8dc9828e03 |

Parent of this evidence-only commit must be141754a; push requires an exact remote141754a lease. No implementation/debug/test edits, PR, merge or deployment. Root owns reading assertions and deciding the next implementation release.
