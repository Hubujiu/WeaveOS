# Independent CSRF secret RED → GREEN

- Oracle: user-approved Redis Session specification section 3 requires an independent 32-byte canonical Base64URL CSRF token, SHA-256 digest stored in the Session, and constant-time server-side binding comparison. These details do not depend on the still-open Q7 Cookie name or Q8 Redis client.
- Pre-implementation snapshot: commit `2cea9f6ec0945b20c722404245520923c72eb32c`; `services/bff/internal/security/csrf_test.go` SHA-256 `86F4E66450F6E8B3180F552FA5685B87907854B52F7A2FC03F2F7027BBB62339`; no-behavior `csrf.go` returned an explicit missing-implementation error/false.
- Windows / Go 1.27.1; command in `services/bff`: `go test ./internal/security -count=1 -v`. Exit 1 at both target assertions: new secret generation returned `CSRF token generation not implemented`, and an independently specified known token/hash pair did not match.
- After implementation, `go test -race ./internal/security -count=1 -v` and `go vet ./internal/security` exited 0. This proves the pure token/hash function; HTTP Cookie/header/Origin integration remains pending Q7 and real end-to-end tests.
