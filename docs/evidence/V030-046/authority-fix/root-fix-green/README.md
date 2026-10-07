# root-fix-green

Root authority fix at 5ab529c619de10ee53f3bbb360d0b66e9b39aa98: targeted WorkflowRead service 8 PASS; HTTPS 4 PASS; full apprecordservice race 174 top-level and 93 subcases PASS. Every command exit 0, no FAIL/SKIP; all stderr empty. Sequential reuse of authorized private PG/Redis. Exact commands, times, source hashes and case results are in original run.json. Source/test/dependency hashes unchanged during execution; tree clean after tests.

TDD:N/A: evidence-only preservation. No executor production/test/debug changes.

| Original artifact | Bytes | SHA256 |
| --- | --- | --- |
| go.mod | 759 | aed642571afb8f38fedf75b2ca402f1464cd577799b9ea8cb7c6fd05072a2632 |
| go.sum | 5039 | 66c155748037a2c08e671e844ab359e500b86775a3304a86bea239835b3da99f |
| race-apprecordservice.exit | 2 | 9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa |
| race-apprecordservice.stderr | 0 | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| race-apprecordservice.stdout.jsonl | 360839 | e118cb76a41849b065d297d07404e62531872dab8986d8d2b9a05078adf30da5 |
| root_workflow_read_http_test.go | 2962 | 52dcb1d50807ad7757ed9f662bcb5ef982eafba441a47f26b58243740456a8b6 |
| root_workflow_read_test.go | 5858 | bd53d7a402ef66ecbe34cc19463c01da620e916ad39d347943534696c4013154 |
| run.json | 55343 | a51d432c447ae0b5c62a0a17cceb66f623d2fafd4ca3e682542527e13b96e4ed |
| workflow-read-https.exit | 2 | 9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa |
| workflow-read-https.stderr | 0 | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| workflow-read-https.stdout.jsonl | 4305 | 66628d4e6587abb972e052b2583a9a9e29b782107b5a03dfbd2e4f40005ea8be |
| workflow-read-service.exit | 2 | 9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa |
| workflow-read-service.stderr | 0 | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| workflow-read-service.stdout.jsonl | 8540 | 4d7715e8d82c9ebd97acfb79fd71240da56592632cdf621607c7561424a69afc |
| workflow_read.go | 10857 | 5916a39be9d8830b17dbaf3ba23208b41602e0d4f81e02456e6c831fd0d13bbf |
