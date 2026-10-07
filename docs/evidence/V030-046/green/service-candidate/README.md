# Original service implementation candidate GREEN

Exact source3c826aae3d2ad88fd127ae1db4824187989926b5. First command seven RootWorkflowRead cases7PASS/0FAIL/0SKIP,exit0. Separate complete apprecordservice default race173top-levelPASS/93subcasePASS,exit0: includes the seven new cases and166oldtop/93oldsub. No skip/filter in full package. Exact argv/environment/times/cases/source hashes in byte-preserved run.json; original JSONL/stderr/exits and source snapshots copied unchanged. Both stderr empty, no race diagnostic. Existing task-private hot/archive PG18.6/Redis8.2.10 reused sequentially, no fixture overlap/reset.

This run precedes the HTTP tests at82b26ef. Source hashes before/after match; later commit adds only Root's HTTP tests and does not change service implementation. No claim that this service result proves HTTP routing, complete BFF/all-repo/product/CI acceptance. No executor implementation/debug/test edits. TDD:N/A for evidence-only preservation.

| Byte-preserved artifact | Bytes | SHA256 |
| --- | --- | --- |
| race-apprecordservice.exit | 2 | 9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa |
| race-apprecordservice.stderr | 0 | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| race-apprecordservice.stdout.jsonl | 359815 | d615753a358daa24f79a6e5874c7d7b1fe4bba668b865f3e6e9de1fa48a68e98 |
| run.json | 58055 | 68fd805407108014bb95f4d3572523317e7c909698f7c86ff643d484dccb3aab |
| workflow-read.exit | 2 | 9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa |
| workflow-read.stderr | 0 | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| workflow-read.stdout.jsonl | 7536 | 589f3100265f9d4d65744ac690183c8ba9db61a3a8166ef75f2154e42ecf3b98 |
| service.go | 5659 | d2cc579654a72da4345677800f76acc3bd769c8684b52982b8501c33534b2612 |
| workflow_read.go | 10764 | 19ded974602e1a8c47025df41b10c7ef4719238193a21bc367371671487db437 |
| root_workflow_read_test.go | 5031 | ff4490e9e868dc82c7f61cccf7e7bbe8993d3da349d75ead58141982591f182b |
