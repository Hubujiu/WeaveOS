# Root HTTPS schema and record-version contract R05 / R06

Runs were made only in the isolated worktree based on commit
`67313718bd43b9d19a0e9b2ceeddb4c0fd9c13e4` and on the dedicated migrated
PostgreSQL/Redis test environment. No production data was read. No credentials
are recorded here.

Root-authored test: `services/bff/cmd/bff/schema_boundary_root_http_test.go`.
Original supplied SHA256: `2fde058b1a964e2a2cd489ce45f812ba21640f5b9ef1e8e087453a1028e5a055`.
Only gofmt whitespace was applied. Final SHA256:
`57f3dcc5544c980857b947c61a78a63bab130e39278d670760e76149271482d3`.
Gofmt-normalized original and working sources had identical bytes and SHA256,
`57f3dcc5544c980857b947c61a78a63bab130e39278d670760e76149271482d3`.
The formatting-only diff is in `root-test-gofmt.diff`.

R05 real verified-HTTPS baseline RED:
`go test -race -count=1 -timeout 2m ./cmd/bff -run
'^TestRootHTTPSRecordVersionContract$' -v` exited1 at the expected first
schema0 write, HTTP400 COMMON_VALIDATION_FAILED instead of409
APPLICATION_SCHEMA_NOT_READY. Verbatim observed result is in `r05-red.log`.

R06 applied exactly four service changes in the Create/Edit input guards and
schema mismatch mappings. For Create and Edit, expectedSchemaVersion now rejects
negative values but admits valid schema0; Edit still requires
expectedRecordVersion>=1. Both actual schema CAS branches now return
APPLICATION_SCHEMA_CONFLICT with currentSchemaVersion from the same loaded facts.
No HTTP/decoder/test semantics or other implementation were changed.

R06 root HTTPS contract command, exit0: `r06-root-https-green.log`.
`go test -race -count=1 -timeout 2m ./cmd/bff`, exit0:
`r06-cmd-bff-race.log`.
`go test -race -count=1 -timeout 2m ./internal/apprecordservice`, exit0:
`r06-apprecordservice-race.log`.

The HTTPS test verifies the server certificate before each status assertion.
Its first not-ready request, schema conflict, normal create/search/detail/edit,
stale row conflict, and no-residue assertions completed. These focused tests do
not constitute full v0.3.0 acceptance or deployment approval.
