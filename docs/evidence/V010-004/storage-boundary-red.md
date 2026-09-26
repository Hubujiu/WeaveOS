# Malformed Session storage: genuine RED → GREEN

Recoverable pre-fix implementation: `session-pre-boundary.go`, exported from `b61fd5338f3cecb0aaa0e5d4c8a9d8535a4a3ff5`, SHA-256 `4ED7028844CAA5FC3A63139B0B04F036C4E6A0777ECD4D2E663980F07AE8005C`. Original no-behavior stub: `session-original-stub.go`, exported from `f0610632448360226cbfd325d2ede256a572d14d`, SHA-256 `A32C8D46C5AD3D4F922B1EE934583818FAD2BF9D6241A90CC39020AAB4FD20CE`. Copy these sources and the corresponding test snapshot back into the original package for a separately labeled replay; do not treat replay as the original run.

Source: approved Redis Session data specification sections 2, 4 and 5 require one valid JSON record, valid timestamps and TTL within (0, 3600000] ms; invalid structure must not be renewed. These are independent storage requirements and do not choose the still-unanswered Cookie names in Q7.

2026-09-26, real local Linux Redis 8.2.1, Go 1.25.7 container, race detector enabled:

- Before fixes, `go test -race ./internal/session -count=1 -run 'TestLoadRejectsTrailing|TestTouchRefusesCorrupted' -v` exited 1 at both target assertions: a second JSON document was accepted; an invalid negative creation timestamp was renewed. Test-first commit `ef9e95a`; recoverable `session-boundary-red_test.go`, SHA-256 `12073CCDF7C7B3EAAA3C850B46804C8B7F66F3891F668C1DE3E7B11E00D2C8C0`.
- Before fixes, `go test -race ./internal/session -count=1 -run TestLoadAndTouchRejectOverlongTTL -v` exited 1: both Load and Touch accepted a 7200000 ms TTL. Test-first commit `ab7d012`; recoverable `session-ttl-red_test.go`, SHA-256 `71657200A1E99CF70E8A51615BAF129B749C787D1C7EA4DD0678584741378B2F`.

All Go commands ran through `docker run --rm --network container:weaveos-v010-004-redis --mount 'type=bind,src=D:\Workspace\WeaveOS-worktrees\V010-004\services\bff,dst=/src' --mount 'type=volume,src=weaveos-v010-go-cache,dst=/go/pkg/mod' -e WEAVEOS_TEST_REDIS_URL=redis://127.0.0.1:6379/15 -w /src golang:1.25.7` followed by the command above. Preconditions loaded and actual Redis operations succeeded, so these are behavior REDs.

After fixes: JSON decoder requires EOF, Load rejects overlong TTL, atomic Lua rejects overlong TTL/invalid timestamps and uses Redis TIME plus monotonic last-seen update as in the source reference. `go test -race ./internal/session ./internal/security ./internal/identity -count=1 -v` exited 0: six SessionStore cases plus all existing security/identity cases passed. Real HTTP, current-user PG validation and Cookie/CSRF integration remain pending; this result does not prove those paths.
