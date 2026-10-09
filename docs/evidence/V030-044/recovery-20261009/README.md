# V030-044 recovery verification

This is a new verification of preserved reservation implementation, not a new public-trigger delivery. Original RED/GREEN evidence remains unchanged in sibling directories.

Local source f360bc9 and remote equivalent d3da67da7cdbacc5a30636248d1048d97bdb1d9b have identical tree 93a41cd291f780611186d8afbe5857672c510ea3. Two new test functions add active/terminal and admission rejection coverage without changing implementation. They are supplemental regressions, not retroactive original RED.

- Focused race run: 8 top-level and 11 subcases pass.
- Corrected UTF8 full relevant-package race run: apprecordservice 237 top/114 sub; appstructure 118 top/29 sub; zero failed or skipped test cases. workflowcatalog reports no test files; its package skip is not a passed test.
- Original SQL_ASCII fixture run retained: existing Unicode account assertion failed. No source/expected value change; recreated database explicitly UTF8 and reran.
- Governance/foundation: 447 pass, 0 fail/skip. Relevant go vet passed.
- All 23 application migrations, archive migration and explicit runtime roles applied to private fresh databases. Services stopped after each run.
- No public API start, trigger configuration, durable start dispatch, real Flowable new-instance end-to-end or final CI completion claim.

Commands: go test -race -p 1 -count=1 -json -run '^TestRootTriggerReservation' ./internal/apprecordservice; go test -race -p 1 -count=1 -json ./internal/apprecordservice ./internal/workflowcatalog ./internal/appstructure. Full run metadata and artifact hashes are in summary.json.

JSONL logs are stored as lossless gzip (mtime=0); raw and compressed hashes are recorded. Initial failed fixture run is preserved, not discarded.
