# V030-047 actual close GREEN and related regression

Root-reviewed production source candidate: 65e7bafcd51081463be4fdfd2044b75484698537. Only workflowprojection/projection.go and close_finalization.go were taken. Original seven tests remain mechanically formatted source 19a9a2ee (formatted SHA256 8a56402baaeba01b9ba30ad1acf3cab825c9d7e057d1ecd8458705e6252d67d0); prior actual five and seven RED evidence remains in ../close-red and ../close-red-seven. Task parent is 3eb029fa.

- Seven real PG tests: go test -race -p 1 -count=1 -json -run '^TestRootWorkflowClose' ./internal/apprecordservice.
- Complete requested regression: go test -race -p 1 -count=1 -json ./internal/apprecordservice ./internal/workflowprojection ./internal/workflowexecution ./internal/workflowcatalog ./internal/flowcommands. All five requested exact package paths exist and were included.
- BFF full-package go vet ./... and go build ./....
- Existing isolated real V045 PostgreSQL verified version22 before/after; existing archive/Redis reused. No migration, new service, credential or autonomous implementation/debug action.

Seven targeted tests PASS. Full regression: apprecordservice 196 top-level / 95 subtests, workflowexecution 9 top-level, flowcommands 67 top-level / 127 subtests; totals 272 top-level / 222 subtests PASS, no failing or skipped tests. workflowprojection and workflowcatalog are included with no test files.

All requested commands exit0; actual per-case and per-package counts and timestamps are in run.sanitized.json, with original stdout/stderr/exit files. 82 code/test/migration/dependency file hashes stayed unchanged throughout execution. Source snapshots and raw outputs are exact byte copies, recorded by ordered [path,digest] pairs in raw-copy-hashes.json. Original run.json stays private; the repository run is explicitly sanitized and is not byte-identical. Environment dumps, credentials and private keys are excluded.

This is the close slice's seven-test and related backend regression evidence. It is not a combined V045/V046 integration run, a formal Java/Flowable runtime loop, final CI or accepted delivery. Root owns those remaining decisions and acceptance. No PR, merge or deployment is created here.
