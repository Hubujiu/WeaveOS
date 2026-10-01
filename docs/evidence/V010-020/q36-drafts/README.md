Q36 draft backend: in progress, not root acceptance.

Base SHA: 1468a969d25d0ba73586db65e83ac21ea8647b20; independent task/V010-020-drafts-backend.
Approved oracle: docs/tasks/V010-020-PLAN.md §5; contracts/query-drafts.md; OpenAPI draft schemas.
Notion PRD 3ec2f5a9e648814e8e5cf89bb46c068d and ADR007 3ec2f5a9e64881c19aa8d5fc6d821ab8 read back 2026-10-01; updated 12:34:16.119Z / 12:31:05.697Z, approvals through current prose agree with PLAN. Native verification is unverified; tool did not report truncation/unknown blocks.
No .agents/skills files were present in the selected workspace; root and BFF AGENTS read.

RED command: `python3 docs/evidence/V010-020/q36-drafts/run.py test test -race -count=1 -run TestDraft -v ./internal/personnel`.
Actual exit 1, red-valid.log: 10 top-level tests reach missing behavior assertions against compile-only ErrNotImplemented placeholders. RED source preserved as *.go.txt and SHA256 manifest.
First run red.log also exposed an invalid department fixture (missing required parent). Fixed fixture against existing schema, then reran before implementation; only red-valid.log counts as complete RED.
Initialization first failed during PG temporary-server transition (environment.log); retry Up successful (environment-retry.log). Neither is behavior RED. Runner now waits for TCP readiness.
Test-only grants on isolated PG: SELECT/INSERT/DELETE drafts and UPDATE(payload_json,draft_version,updated_at). Production role grants, shared routes/constructor, formal write cleanup call sites remain integrator-owned.
No migration Down or production access. Effective model metadata is unknown: no tool supplied it.
