# V030-047 actual close RED

Root-authored source commit: 60ef37f5473320cba9ccccc61ce27176ed08ba76, base b056fc34f41d389c5a4aee54b6e81a29dae578bd. Only mechanical gofmt was applied before execution. Original and formatted source snapshots are retained.

Actual command: go test -race -p 1 -count=1 -json -run '^TestRootWorkflowClose' ./internal/apprecordservice. Existing real isolated V045 PostgreSQL was verified version22 before and after; no migrations or new services. Result: exit1, 4 FAIL and 1 PASS, no SKIP. Three failures reached last-confirmation assertions with state closing/revision4 rather than disabled/revision5; the fault test got nil instead of its real close-write trigger error. Completion without a close request passed. This is behavior RED, not compilation/environment failure.

Raw stdout/stderr/exit and both test snapshots are exact bytes copied from the original private execution. run.sanitized.json is explicitly sanitized and is not byte-identical to the original private run.json. Raw-copy hashes are ordered [path,digest] pairs. No environment dump, credentials, private keys, new implementation, test changes beyond gofmt, or DDL modifications are included. Root owns further tests, debug, implementation contracts and acceptance.
