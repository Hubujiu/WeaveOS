# Q13 · printable ASCII password boundaries

2026-09-26 actual Notion answer: “都只允许标准 ASCII 可打印符号”. Official PRD/ADR-001/auth feature/database design/credential object were updated and re-fetched (03:46 UTC), then Figma Register 40:2 received the matching native annotation. Approval states remain unchanged. Printable means U+0020–U+007E; space is allowed but not a special class, non-ASCII/control characters are rejected, no trimming or extra minimum length.

## RED before implementation

Immutable test commit e42fd88; test and implementation snapshots are password-ascii-red-test.go/password-ascii-before.go. Linux Docker golang:1.25.7 with real PostgreSQL 18 / Redis 8.2, isolated weaveos_ci_test / Redis DB15:

`go test -race ./internal/auth -run 'TestPasswordPrintableASCII|TestRegistrationRejectsNonPrintableASCII' -count=1`

Actual exit 1: six nonconforming password fixtures accepted by policy; registration containing non-ASCII returned 201 instead of 400. The registration failure happens at the first fixture; later rollback assertions were not yet reached and are not claimed as RED.

## GREEN and regression

Minimal policy change rejects characters outside U+0020–U+007E before class counting. No password normalization. Actual `go test -race -p 1 ./... -count=1`, `go vet ./...`, `go build -o /tmp/bff ./cmd/bff` all exit 0 in the same isolated environment. Registration rejects malformed passwords without consuming invitation; printable-space password registers/logs in exactly, trimmed variant fails. All prior storage/HTTPS/audit/reset race tests remain green.

Full product three-browser tests and archive/recovery still belong to 007/008; no whole-version success is asserted here.
