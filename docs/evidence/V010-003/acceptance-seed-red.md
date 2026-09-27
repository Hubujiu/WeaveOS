# Acceptance Seed RED

- Requirement: V010-003 acceptance seed must produce isolated, private fixtures backed by real PostgreSQL rows.
- Test snapshot: commit `dfd5323`; `services/bff/cmd/acceptance-seed/main_test.go` SHA-256 `4AE913CA72B3E7E9218EE3D9366914C26DEC8B5ABC6024E62DC643F38BA4EFE8`.
- Environment: Windows, Go 1.27.1, PostgreSQL 18.4 on loopback `127.0.0.1:55483`, isolated database `weaveos_v010003`, migration version 1 applied. URL held in `WEAVEOS_TEST_DATABASE_URL`; no credentials or fixture values are recorded here.
- Command from `services/bff`: `go test ./cmd/acceptance-seed -run TestAcceptanceSeedProducesIsolatedUsableFixtures -count=1 -v`.
- Actual exit: 1. Target assertion failed: `isolated acceptance seed must run: acceptance seed not implemented`. The package compiled, connected prerequisites were available, and this is the required missing behavior.

## Additional guard and correction

- Initial seed implementation produced usable database fixtures but the Unix permission-bit assertion failed on Windows. Go `os.Chmod` documentation states Windows only uses its owner-writable bit, so the assertion did not measure ACL privacy. The test was corrected to inspect `icacls`; implementation now removes inherited ACLs and grants the current user full control before writing secrets. This is a test-oracle correction for the host OS, not a weakened privacy requirement.
- The test now clears only an explicitly supplied `weaveos_` isolated database before and after its fixture run, making repeated local runs independent. It refuses fixture cleanup against other database names.
- Separate test snapshot commit `89d7c63`, test SHA-256 `2BFCF786BD97E2E42A86BD952D8D83182B2B28D7FE6A9948A2EB18CE7C0992D1`. Command: `go test ./cmd/acceptance-seed -run TestAcceptanceSeedRejectsNonTestDatabaseBeforeMutation -count=1 -v`; exit 1 at target assertion, because the seed reached a missing `auth.users` table in the default `postgres` database instead of rejecting that database before access. The test database was not modified.
- After the guard implementation, `go test -race -p 1 ./... -count=1`, `go vet ./...`, and `go build ./...` all exited 0 in `services/bff` with `WEAVEOS_TEST_DATABASE_URL` set to the isolated PostgreSQL 18.4 database. Package order `-p 1` prevents independent packages from truncating the same isolated database concurrently.
