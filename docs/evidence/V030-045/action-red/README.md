# Original lifecycle Accept and operation-kind constraint RED

Exact source2ae9feb4175573f7abcd44470ab9a69c1d46938d. Frozen `go test -race -p 1 -count=1 -json -run '^TestRootLifecycle(Action|Operation)' ./internal/apprecordservice`: exit1,6top-levelFAIL/6subcaseFAIL,0PASS/0SKIP. Compiles and completes real PG/Redis preview fixtures; empty stderr/no race diagnostic. Existing migration1..21 reused, no022 applied or authored by executor.

| Case family | Exact failure |
| --- | --- |
| AcceptedIdentityAndNoEarlyTransition | withdraw/return bothline40 ErrUnavailable |
| ReplayAndChangedPayload | line65 first Accept ErrUnavailable |
| RejectsUnvisitedTargetWithoutWrites | line83 actual ErrUnavailable, expected ErrWorkflowTaskChanged |
| LatestBasisAndRevokedPermission | version/revoke bothline110 ErrUnavailable, expected basis-changed/denied respectively |
| WriteFailureRollsBackEverything | line130 did not reach actual injection: ErrUnavailable; no rollback proof claimed |
| OperationKindsAndClosedReceipt | both new kindsline155 ck_operation_kind CHECK SQLSTATE23514; later malformed-receipt assertion unreached |

AcceptUnavailable reflects the declaration-only missing behavior. New-kind CHECK refusal is intended DB-contract RED before Root's022 migration, not a connection/role/compiler/environment failure. Original full JSONL/run.json/stderr/exit and immutable test/stub snapshots copied byte-for-byte. Exact SDK argv/environment/times/source/artifact hashes in run.json; before/after hashes match. Same private hot/archivePG18.6/Redis8.2.10 reused sequentially after V046 unrelated-edit reproduction; V046 remains separate and is not mixed here. No debug/implementation/test/migration/runner/security change, PR/merge/deployment. Root owns raw assertion review and subsequent release. TDD:N/A for evidence-only preservation; parent2ae9feb with exact remote lease.

| Byte-preserved artifact | Bytes | SHA256 |
| --- | --- | --- |
| run.json | 6232 | 0f5dd74c12f3f381f1a5399fe07f7f20e6dad578402f6e81300aa7280999f872 |
| test.exit | 2 | 4355a46b19d348dc2f57c046f8ef63d4538ebb936000f3c9ee954a27460dd865 |
| test.stderr | 0 | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| test.stdout.jsonl | 16749 | ff4a03faf16d63a67e10a20c39c380035b106aabc5e17bae19f7171c3b36b241 |
| workflow_lifecycle_action.go | 541 | 4cbdb7f09a9bc56cf0034932926069deb0a48fe06ce2d7a1e6511d901f8e3a09 |
| root_workflow_lifecycle_action_test.go | 7145 | 18ea399fb34faf9499d4276a4142bdfa4463b05e88c7970c149184d5d21f5bd0 |
