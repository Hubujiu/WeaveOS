# Replay fix GREEN and catalog scale failure

Exact frozen source: `44b2058ac2ff4de107e2ae9a0f20cc4cbb230fa1`. Executor changed no implementation, test, dependency, fixture, timeout, pool or planner policy. All execution hashes unchanged. Full commands, times, test names, source hashes and artifact SHA256 in run.json; JSONL/stderr/exit are byte-preserved original logs.

`go test -race -p 1 -count=1 -json -failfast -run ^TestRootWorkflowSave ./internal/apprecordservice`: exit0,20 top-level PASS/2 subcase PASS, including unchanged three-second one-connection replay and Root's current-read-revocation protection.

`go test -race -p 1 -count=1 -json -failfast ./internal/apprecordservice`: exit1,91 top-level PASS/59 subcase PASS/1 top-level FAIL,0SKIP. First failure TestRootCatalogCompatibilityDoesNotScanArchivedVersions at root_workflow_catalog_scale_test.go:118: aggregate_plan_rows=298 exceeded256. Remaining commands were stopped at that point. Empty stderr, no race diagnostic. These partial passes are not full package acceptance.

Complete original failure output and EXPLAIN extracted from existing JSONL into catalog-failure.output.txt and catalog-failure.explain.json; test/EXPLAIN not rerun. Observed version Index Scan uq_workflow_publication_version returns1 row with1 loop. Other plan branches include workflow_definitions Seq Scan with147 rows removed and workflow_instances Seq Scan with144 removed. Root owns diagnosis; threshold/oracle/source unchanged.

Read-only facts collected2026-10-07T03:05:54Z before authorized continuation: total workflow_versions5150; failure test app dfc07ba8-b019-4c28-a931-b8413addb2a9 has5000 versions/1flow. Exact SQL and psql stdout/stderr/exit retained. Explicit last_analyze: definitions03:00:04.092625Z, versions03:00:04.187126Z, instances03:00:04.188457Z. pg_stat estimates and automatic analyze/vacuum timestamps retained as observed, not claimed historical failure snapshots. No executor ANALYZE/VACUUM, EXPLAIN rerun or planner-setting change; original frozen scale test itself executes ANALYZE.

Root separately authorized continuation of the other four default packages, vet and build after this recorded failure. They will have separate evidence; overall remains blocked by this scale assertion and pending formal engine acceptance. No PR, merge or deployment.
