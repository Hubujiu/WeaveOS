# Last two PRD contracts: frozen three-run GREEN

Exact source96749d9a6d5b37de5fb40c52ba59e7b46a7915c1. Root added only two tests to root_workflow_save_test.go; existing tests and production unchanged. Executor executed without implementation/test/debug changes. Existing isolated real PG18.6/Redis8.2.10 and archive database reused, no fixture reset. All before/after hashes unchanged; exact argv/times/case names/source/artifact hashes in run.json.

| Run | Exit | Top-level PASS | Subcase PASS | FAIL/SKIP |
| --- | --- | --- | --- | --- |
| Exact new two cases | 0 | 2 | 0 | 0/0 |
| All TestRootWorkflowSave | 0 | 22 | 2 | 0/0 |
| Entire apprecordservice default race | 0 | 189 | 95 | 0/0 |

All commands use `go test -race -p 1 -count=1 -json -failfast`, in the above order. Empty stderr and no race diagnostic. New tests verify A Save leaves sibling B's task independent, B reads shared v2 and rejects its old basis; simultaneous Save/Agree with one basis has exactly one successful operation, preserving the corresponding record/fence/version semantics. No claim that both race winners were deterministically forced.

Other four packages/vet/build were deliberately not repeated after this test-only change per Root instruction. Their full defaults and six formal-runtime exact identities passed at0987d377, with unchanged production/other tests; actual-green preserves those original reports and commands. Prior RED, replay resource-boundary defect/fix, old scale-oracle failure/full EXPLAIN and original source preserved. Latest GitHub CI and final source/evidence acceptance remain pending Root review; no main, deployment or merge.
