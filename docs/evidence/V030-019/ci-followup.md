# V030-019 CI follow-up

## Failures on Root-registered HEAD

- Branch `task/V030-019-flow-graph`, PR #29 draft, base `task/V030-012-original-app-shell`.
- Root-registered head: `962e15ef179e76480d73255ec8ed9e30f4385113`.
- GitHub CI run `37177585634`, created `2026-10-04T04:38:23Z`, was still in progress while these failed jobs had completed. GitHub API confirmed both jobs were for the above HEAD.
- Job `111363359222` (`governance`) failed at step 6, `Run node scripts/check-tasks.mjs`; 240 governance/foundation checks and `node scripts/verify-repo.mjs` had succeeded. The task checker reported `docs/tasks/V030-019.md: [TASK] missing Handoff`.
- Job `111363359286` (`go`) failed at step 7, `Format, static checks, race tests and build`. The required format gate found `services/bff/internal/flowgraph/engine_export_test.go` unformatted, so later Go checks in that composite step did not run on this head.

## Minimal repairs

- Ran standard `gofmt -w services/bff/internal/flowgraph/*.go`. It changed only formatting in Root-authored `engine_export_test.go`; the before/after SHA-256 is `fcaa0f596590494dc6f37e48d9362585909ed525ebbd1d52391a7cedfb267e1d` → `d5e26e0289a89d3e813e11a928968e50df722684eda2125edcefcd9829bbe968`. The other four package Go files had identical hashes before and after. `gofmt -l services/bff/internal/flowgraph/*.go` is empty.
- Added the required `## Handoff` task section: graph validation, BPMN compilation, and limited real-engine verification are complete; Root owns the next definition-storage, controlled-RPC, and business-permission wiring steps; full v0.3 remains incomplete and this task does not authorize a main merge or deployment.
- Updated the stale third-package task note to point to the already-retained real engine reports; no behavior, test logic, CI gate, or XML was changed.

## Verification on the repair candidate

- `gofmt -l services/bff/internal/flowgraph/*.go` — no output; exit `0`.
- `GOCACHE=/tmp/v030-019-go-build-cache GOMODCACHE=/workspace/.weaveos-tools/go-mod /workspace/.weaveos-tools/go/bin/go test -race ./internal/flowgraph` — all 22 tests passed; exit `0`.
- `GOCACHE=/tmp/v030-019-go-build-cache GOMODCACHE=/workspace/.weaveos-tools/go-mod /workspace/.weaveos-tools/go/bin/go vet ./internal/flowgraph` — exit `0`.
- `node scripts/check-tasks.mjs` — `Task documents and current PR scope are structurally valid.`; exit `0`.
- `node scripts/verify-repo.mjs` — `Repository structure checks passed (not a business/TDD/Notion verification).`; exit `0`.
- `git diff --check` — exit `0`.
- These local results were obtained before committing the repair. PR #29 CI rerun on the repair commit is pending.

## Full-repository formatting inventory

The package is clean. Repository-wide `gofmt -l` reports 35 out-of-package evidence files; none were modified:

```text
docs/evidence/V010-004/authenticate-before-renew.go
docs/evidence/V010-004/cookie-stub.go
docs/evidence/V010-004/request-red_test.go
docs/evidence/V010-004/authenticate-boundary-red_test.go
docs/evidence/V010-004/authenticate-stub.go
docs/evidence/V010-004/authenticate-red_test.go
docs/evidence/V010-004/session-ttl-red-test.go
docs/evidence/V010-004/cookie-red-test.go
docs/evidence/V010-004/session-boundary-red-test.go
docs/evidence/V010-004/request-stub.go
docs/evidence/V030-013/schema-conflict-http-probe.go
docs/evidence/V010-019/personnel-schema-red-test.go
docs/evidence/V010-007/auth-before-validation-proxy.go
docs/evidence/V010-007/redis-deadline-red-test.go
docs/evidence/V010-007/challenge-red-test.go
docs/evidence/V010-007/validation-red-test.go
docs/evidence/V010-007/proxy-config-red-test.go
docs/evidence/V010-007/config-before-proxy.go
docs/evidence/V010-007/proxy-red-test.go
docs/evidence/V010-005/config-env-red-test.go
docs/evidence/V010-005/http-red-test.go
docs/evidence/V010-005/business-dispatch-red-test.go
docs/evidence/V010-005/seed-red-test.go
docs/evidence/V010-005/password-ascii-red-test.go
docs/evidence/V010-005/invitation-stub.go
docs/evidence/V010-005/lifecycle-red-test.go
docs/evidence/V010-005/config-env-stub.go
docs/evidence/V010-005/invitation-red-test.go
docs/evidence/V010-005/audit-boundary-red-test.go
docs/evidence/V010-005/http-protocol-red-test.go
docs/evidence/V010-005/composition-red-test.go
docs/evidence/V010-005/composition-stub.go
docs/evidence/V010-005/lifecycle-stub.go
docs/evidence/V010-005/http-corrected-fixture-test.go
docs/evidence/V010-005/platform-before-dispatch.go
```

## Result boundaries

The repair changes only task documentation and whitespace in `engine_export_test.go`. The existing generated BPMN resources, Java tests, engine evidence, compiler, validator, and CI workflow are unchanged; no Maven/Flowable run was repeated for formatting/documentation-only changes.
