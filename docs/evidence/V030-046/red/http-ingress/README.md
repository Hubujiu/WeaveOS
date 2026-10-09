# Original real HTTPS route-missing RED

Exact source82b26ef09ca8561cd2718a6066e0bbee2755bc83, adding only Root's four HTTPS tests. Exact command `go test -race -p 1 -count=1 -json -run '^TestRootWorkflowReadHTTPS' ./cmd/bff`: exit1,4FAIL/0PASS/0SKIP. Compiles and creates real HTTPS/session/PG fixtures; first valid authenticated request in each case returns404 API_NOT_FOUND, expected200. Failures at root_workflow_read_http_test.go lines15,33,47,60. Later body/auth/CSRF/actor/refresh assertions have not executed yet. Empty stderr, no race report, no environment/fixture failure. Root owns actual RED review and the next implementation release.

Original JSONL/stderr/exit/run.json and frozen HTTP test source preserved byte-for-byte, before/after hashes unchanged. Same isolated hot/archive PG18.6/Redis8.2.10 reused only after V044 six-case RED finished. V044 logs remain private/on its own branch and are not mixed here. No production/test/debug changes. TDD:N/A for evidence-only preservation.

| Byte-preserved artifact | Bytes | SHA256 |
| --- | --- | --- |
| run.json | 4544 | e19863a99a52d91ef7a2c58495674d03638eb7abc4ea8ba34f5f4e8c1cbb46aa |
| test.exit | 2 | 4355a46b19d348dc2f57c046f8ef63d4538ebb936000f3c9ee954a27460dd865 |
| test.stderr | 0 | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| test.stdout.jsonl | 5958 | 0af969c39ae085ef50e1bbae872b491223f378466abfadf203ba9ff0931091e1 |
| root_workflow_read_http_test.go | 2962 | 52dcb1d50807ad7757ed9f662bcb5ef982eafba441a47f26b58243740456a8b6 |
