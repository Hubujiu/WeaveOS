# V030-019 RED/GREEN evidence

## Source and environment

- PRD fetched from Notion page `V030-019 PRD｜受控流程图与发布校验` at `2026-10-04T03:48:54.080Z`; page path: project → PRD → iterations → `v0.3.0 应用导航、数据表单与审批流程`; page verification state: `unverified`; response had no truncation indicator.
- ADR fetched from Notion page `V030-019 ADR｜流程图结构与统一条件校验` at `2026-10-04T03:48:55.141Z`; page path: project → Architecture & ADR → ADR database → `ADR-009 v0.3.0 应用导航、数据与流程架构（提案）`; page verification state: `unverified`; response had no truncation indicator. The task-specific text labels the V030-019 contract as frozen.
- Go: `go version go1.27.1 linux/amd64` (`/workspace/.weaveos-tools/go/bin/go`).
- Build cache: `GOCACHE=/tmp/v030-019-go-build-cache`; module cache reused from `/workspace/.weaveos-tools/go-mod`.
- Worktree: `/workspace/WeaveOS-worktrees/V030-019`, branch `task/V030-019-flow-graph`, starting commit `0aac9a38576a28c97cd5f8d01e7c05cd411a50a8`.

## Mechanical formatting before RED

`gofmt -w services/bff/internal/flowgraph/graph.go services/bff/internal/flowgraph/graph_test.go` was run before behavior testing. It only changed Go formatting; no test assertions or logic were edited.

| File | SHA-256 before gofmt | SHA-256 after gofmt |
| --- | --- | --- |
| `graph.go` | `460d54047ef5aee154d302b6893ea99842340ae28f5060323921233d9cf8fca7` | `1a7f2cae33bbcf02c8acec0307d533c551fec2b1c8e5a48f95ff251c4b3eca0a` |
| `graph_test.go` | `f8238129782f1e3b2509705cafe19fff36aebb99ab5fdf80c415ab96b3006560` | `b6009a70e1bbe9e3f737fed674c28bbd8148e60d1042866a055ca2b5b1ada4b7` |

## RED

- Command, from `services/bff`: `GOCACHE=/tmp/v030-019-go-build-cache GOMODCACHE=/workspace/.weaveos-tools/go-mod /workspace/.weaveos-tools/go/bin/go test ./internal/flowgraph`
- Exit code: `1`.
- The package compiled and all 12 root-authored test functions failed against the no-behavior placeholder. The valid-graph cases failed on the placeholder error; invalid graph/configuration cases showed invalid input being accepted because the placeholder error did not wrap `ErrInvalid`. This is behavior RED, not a compilation or dependency failure.
- Test SHA-256 at RED: `b6009a70e1bbe9e3f737fed674c28bbd8148e60d1042866a055ca2b5b1ada4b7`.
- The first attempted command omitted `GOMODCACHE` and exited `1` before compilation because Go tried to create `/home/agent/go` on the read-only filesystem. It is excluded from RED evidence. The command above reran with the existing module cache and reached the tests.

## GREEN

Implementation is present in `services/bff/internal/flowgraph/graph.go` (SHA-256 `7d4a76f88e846cf27407a38eb0e931493e0a6afa070b8148f4ca3662adab46b1`).

- Race command, from `services/bff`: `GOCACHE=/tmp/v030-019-go-build-cache GOMODCACHE=/workspace/.weaveos-tools/go-mod /workspace/.weaveos-tools/go/bin/go test -race ./internal/flowgraph`
- Exit code: `1`. Eleven of twelve test functions pass. The sole failure is `TestRootDeterministicAndNoInputMutation` at `graph_test.go:194`, reporting `input mutated`.
- Root cause verified without editing the tests: the test's `clone` helper JSON-marshals a Graph then unmarshals it. Nil `json.RawMessage` fields become the bytes `null`, so `reflect.DeepEqual(g, before)` is already false before calling `Validate`. The check therefore cannot currently prove input mutation. `Validate` does not write to its inputs; changing nil input fields to `null` to satisfy this assertion would violate the contract.
- The test file remains unchanged after the pre-RED gofmt snapshot: SHA-256 `b6009a70e1bbe9e3f737fed674c28bbd8148e60d1042866a055ca2b5b1ada4b7`.
- Static check: `GOCACHE=/tmp/v030-019-go-build-cache GOMODCACHE=/workspace/.weaveos-tools/go-mod /workspace/.weaveos-tools/go/bin/go vet ./internal/flowgraph` exited `0`.
- Diff check: `git diff --check` exited `0`.
- Full GREEN remains blocked until Root corrects the nil-preserving test clone (without weakening its assertion) and the full race command passes. No test logic was changed in this worktree.
