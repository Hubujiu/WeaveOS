# Redis Session test-first evidence

## Immutable test snapshot

- Source: current v0.1.0 PRD FR-005/007/008/009, accepted ADR-001/002, user-approved Redis Session data specification (key/JSON/TTL/touch/DEL). Cookie naming and production Redis Go client remain pending Notion Q7/Q8; these tests cover the independent storage behavior only.
- Test-first branch commit `f0610632448360226cbfd325d2ede256a572d14d` contains `services/bff/internal/session/store_test.go` SHA-256 `62DFA4886BA81496F35718BDFD2688597F808DEA2EAB5305C0752AA544193094` and an explicitly no-behavior `store.go` stub. The tested interface was declared only to compile the tests; it does not perform Redis operations.
- Cases: create/load exact approved Redis key and JSON facts with approximately 3,600,000 ms TTL; separate random SID and CSRF token; atomic sliding touch; logout deletion; concurrent touch/revoke cannot restore the key.

## CI environment preparation RED

[PR #8 CI run 36119333314](https://github.com/Hubujiu/WeaveOS/actions/runs/36119333314) at head `f061063` ran the Go test package but lacked an isolated Redis service and `WEAVEOS_TEST_REDIS_URL`. All three tests exited 1 at the environment precondition. This is evidence of missing CI preparation, **not** valid Session behavior RED. The local Redis installation is version 3.0.504 and likewise cannot stand in for Redis 8.2. The next step adds Redis 8.2.1 to the CI Go job, then re-runs the same immutable test snapshot to reach the storage behavior assertions.

## Valid storage behavior RED

[PR #8 CI run 36119977732](https://github.com/Hubujiu/WeaveOS/actions/runs/36119977732), head `f114afa4f596346c313feac82b6e402e71ece527`, ran the same unchanged `store_test.go` against the added real Linux Redis 8.2.1 service, database 15. The Redis version precondition and network setup passed. Go job exited 1 at the target `Store.Create` calls in all three tests: `session storage not implemented`. The genuine no-behavior store stub and test source are still pinned in commit `f061063`; no Redis behavior has yet been implemented. Browser smoke and governance jobs passed. This is the required pre-implementation Session behavior RED.

## GREEN after Q8 source sync

2026-09-26: User answered Q8 `go-redis/v9`; ADR-003 was updated and reread without changing its proposed status. `go-redis/v9 v9.22.0` was pinned in `go.mod`/`go.sum`; its module requires Go 1.24, while CI uses `.go-version` 1.27.1. The test-first source remains the immutable `f061063` snapshot above.

Local Docker Desktop Linux daemon 29.6.2 ran `redis:8.2.1` as `weaveos-v010-004-redis` on host loopback port 6388. `docker exec weaveos-v010-004-redis redis-cli INFO server` reported `redis_version:8.2.1` and Linux/WSL2. Host PowerShell lacks a Go executable, so Go 1.25.7 was run in an isolated Linux container sharing the Redis container's network and bind-mounting this task worktree. Command: `docker run --rm --network container:weaveos-v010-004-redis --mount 'type=bind,src=D:\Workspace\WeaveOS-worktrees\V010-004\services\bff,dst=/src' --mount 'type=volume,src=weaveos-v010-go-cache,dst=/go/pkg/mod' -e WEAVEOS_TEST_REDIS_URL=redis://127.0.0.1:6379/15 -w /src golang:1.25.7 go test -race ./internal/session -count=1 -v`. Exit 0; all three real-Redis tests passed. `go vet ./internal/session ./internal/security ./internal/identity` in the same Go image exited 0; `git diff --check` exited 0. This proves the isolated SessionStore behavior under these cases, not Cookie/CSRF, PostgreSQL identity validation, or full HTTP authentication.
