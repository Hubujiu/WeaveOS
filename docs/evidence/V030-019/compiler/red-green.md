# V030-019 controlled BPMN compiler evidence

## Frozen sources

- The Notion ADR page `V030-019 ADR｜流程图结构与统一条件校验` was fetched again at `2026-10-04T04:02:01.091Z`; page path is project → Architecture & ADR → ADR database → ADR-009 proposal. Verification state is `unverified`; response showed no truncation indicator. The added “第二包：受控 BPMN 编译” defines the exact mapping and security boundaries.
- The matching frozen implementation plan is `docs/evidence/V030-019/compiler/implementation-plan.md`.
- Go: `go version go1.27.1 linux/amd64`; `GOCACHE=/tmp/v030-019-go-build-cache`; reused `GOMODCACHE=/workspace/.weaveos-tools/go-mod`.

## Root test formatting

Only `gofmt` was run on Root-authored tests. No test logic was changed.

| File | SHA-256 before gofmt | SHA-256 after gofmt |
| --- | --- | --- |
| `graph_test.go` | `441cc9e6e478f25d3197fcfce247617759da131eb58b97c3a54b93c378434b15` | `ba729753eb8e479385011a247c392fd6002d79b362c2766b0fd6a0da151de842` |
| `compiler_test.go` | `a9f13dcd846040ff8d91bd6568e65595381b36f8572fdeb6864528e9e14de7f1` | `2fd2722e93867bd4651b9bf59c900742d72b5bb03f4022b9762eca33cce31f2e` |

## Compiler RED

- `compiler_test.go` SHA-256: `2fd2722e93867bd4651b9bf59c900742d72b5bb03f4022b9762eca33cce31f2e`.
- Before implementing `compiler.go`, command from `services/bff`:
  `GOCACHE=/tmp/v030-019-go-build-cache GOMODCACHE=/workspace/.weaveos-tools/go-mod /workspace/.weaveos-tools/go/bin/go test ./internal/flowgraph -run '^TestRootBPMN'`
- Exit code: `1`. The package compiled and all 9 BPMN test functions failed against the no-behavior placeholder; failure output reported the placeholder error or invalid definition IDs accepted due the placeholder not wrapping `ErrInvalid`.

## Compiler implementation and GREEN

- `compiler.go` SHA-256 after gofmt: `a936c8754d1aa187a6245654b317bae8fb750a6c9d2d34133d7c8eefbe2f95dc`.
- It validates with `Validate`, checks the non-zero canonical definition UUID, and emits fixed XML through `encoding/xml`. Only generated IDs and fixed expressions are serialized. Approval IDs, editable fields, node labels, and condition values are not embedded. No deployment or Flowable call is made.
- Focused compiler tests passed: `GOCACHE=/tmp/v030-019-go-build-cache GOMODCACHE=/workspace/.weaveos-tools/go-mod /workspace/.weaveos-tools/go/bin/go test ./internal/flowgraph -run '^TestRootBPMN'` (exit `0`).
- Full 21-test race run passed: `GOCACHE=/tmp/v030-019-go-build-cache GOMODCACHE=/workspace/.weaveos-tools/go-mod /workspace/.weaveos-tools/go/bin/go test -race ./internal/flowgraph` (exit `0`).
- Static analysis passed: `GOCACHE=/tmp/v030-019-go-build-cache GOMODCACHE=/workspace/.weaveos-tools/go-mod /workspace/.weaveos-tools/go/bin/go vet ./internal/flowgraph` (exit `0`).
- `git diff --check` exited `0`.
- This validates XML generation and mapping only; a real Flowable deployment and approval execution remain separate acceptance work.
