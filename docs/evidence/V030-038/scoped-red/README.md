# Scoped compiler declaration-stage RED

Source: approved V030-038 PRD/ADR and FLOW-13 linked in task document.
Base: 0372d8674b47525dd03a9e3285130a6048ca3a00.
Runtime: official SHA-verified Go 1.27.1 linux/amd64, GOTOOLCHAIN=local.
Command (services/bff): `go test -json -count=1 -run '^TestRootScopedCompiler' ./internal/flowgraph`
Observed: 2026-10-06 01:59:42 UTC, exit 1.
Two failing cases: stable identity across revisions; cross-application isolation. Both fail because the declaration-only stub returns ErrInvalid for valid input.
Two passing cases: invalid scope rejection; unchanged legacy entry point.
No syntax/dependency/setup failure counted as RED. Original JSONL, stderr, exit, independent tests and stub are preserved alongside this file.
Implementation has not begun. V030-037 dependency acceptance remains pending under repository workflow rules.
