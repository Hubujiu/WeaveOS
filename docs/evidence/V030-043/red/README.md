# V030-043 frozen test attempt — dependency blocked

Source: `85c471d9a733f1c08ecacc7913f2e761cc5b633c`. Both Root frozen file hashes match before and after execution. No implementation/test/fixture was changed.

Real PostgreSQL 18.6 and Redis 8.2.10 were prepared using the CI-pinned image digests, `--pull never`, `--network none`, private Unix sockets and no published ports/password. Existing Goose 3.28.0 applied all original app migrations; the original `infra/runtime/roles.sql` succeeded. Unknown pre-existing containers were untouched.

Command: `go test -race -p 1 -count=1 -json -run '^TestRootWorkflowSave' ./internal/apprecordservice` with existing Go 1.27.1, CGO enabled, local toolchain and offline module lookup. Actual exit **1**.

No test body ran. `google.golang.org/grpc v1.84.0` is absent from the existing cache (cached older version is 1.83.2). Compiler setup failed at `internal/workflowrpc/pb/deployment_grpc.pb.go:11:2`: `module lookup disabled by GOPROXY=off`. This is **NOT behavioral RED**. Root's 18 top-level cases and two basis subcases have no assertion results yet. No version substitution or dependency download was performed.

Original stdout JSONL, stderr, exit and migration/role logs are preserved. Submitted log bytes were inspected for credential/DSN/private-path markers and required no redaction; hashes are recorded in `run.json`. Private run metadata/full Docker inspect/container logs remain outside Git. Public metadata replaces host paths, socket DSNs and unique run IDs with descriptive values. No unrelated environment variables were captured.

The isolated containers are intentionally retained for the pending test rerun; no unknown resource cleanup was performed. Next: obtain authorization to fetch the existing locked missing modules (or an authorized exact cache), then rerun unchanged frozen tests. Root owns RED interpretation and any test repair.
