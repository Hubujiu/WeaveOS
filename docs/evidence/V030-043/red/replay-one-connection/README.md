# Root's independent one-connection replay defect

Exact source `32f4b88085a0d2129023289ef24a07988a742428`, parent `dbdb67f3f444d6fbda34e121196870b3501534fe`. Safe fast-forward added only Root's root_workflow_save_replay_test.go; executor prose corrections retained, no source/test/fixture repair.

Frozen new test SHA256: `7ed78534662bc5ea4dc0d1fe68ec74e07ca2cedb72b97ce71d87927ad80d38c5`, unchanged before/after. Existing synthetic task fixture and real PG/Redis reused.

Actual command: `go test -race -p 1 -count=1 -json -run '^TestRootWorkflowSaveReplayUsesOnePoolConnection$' ./internal/apprecordservice`. Exit1,1FAIL/0PASS/0SKIP; stderr empty, no race diagnostic. First Save succeeded; confirmed same-operation replay with MaxConns1 reached Root's assertion after3.12s:

```text
root_workflow_save_replay_test.go:30: confirmed replay must complete without a nested pool acquisition: context deadline exceeded
```

This is an actual behavior/resource-boundary failure after fixture/first-Save success, not a compiler/storage/network error. Current production code still calls GetRecord while the RecordWrite retains its pool connection. Root owns diagnosis and any fix; executor did not alter implementation, test, timeout or connection policy. No further verification command ran after this failure.

Original JSONL/stderr/exit preserved byte-for-byte, hashes and exact environment/source identities in sanitized run.json. Full original connection metadata remains task-private. Broader default-package GREEN happened on the preceding head and must not be used to claim this head is green. Waiting for Root's debug contract.
