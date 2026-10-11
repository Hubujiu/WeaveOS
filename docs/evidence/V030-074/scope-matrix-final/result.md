# Final scope matrix extension

Actual command: `go test -race -count=1 -json ./cmd/bff -run '^TestRootStructureCopy'`, fixed Go1.27.2, PostgreSQL18.6/auth_app and Redis8.2.10, isolated synthetic fixture. Exit0;18 top-level tests and59 subtests passed, no skips. Source-head plus tests.patch reconstructs this test-only extension.

PRD final acceptance recheck added explicit opposite-direction A→B / B→A concurrent copies: both succeed without deadlock; each target advances once to structureVersion2 with exactly2 tables and2 views. Added group-insert failure to form/workflow/audit/receipt rollback matrix, asserting the precise injected failure and no residual state.

No runtime behavior changed. This is supplementary acceptance coverage, not a new RED→GREEN claim. The full-corrected runtime regression remains applicable; final exact remote head CI is still required.
