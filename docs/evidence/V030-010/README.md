# V030-010 B4b evidence

Bounded source: prd-b4b-plan.md (edited 2026-10-03 00:45:42.879 UTC) and
adr009-b4b-plan.md (00:45:56.302), fetched completely through authorized Notion.
Root released the gate in the delegation after those writes/readbacks.

## Authentic TDD

- RED 00:55:27.000–00:55:33.647 UTC, go test -count=1 -json
  ./internal/apppolicy, exit 1: 21 fail and 69 pass test/subtest events. Package
  compiled; failures reached missing allow/projection assertions in stubs.
- RED commit: 6b54bbdacc23f786bff61f89b59fcc979aa83586. red-sources.tar.gz contains
  types, unchanged tests and return-false/return-nil no-behavior stubs. red-run.json
  records source/archive hashes, command, environment and exact failure names.
- GREEN 00:57:40.252–00:57:40.490 UTC, same command, exit 0: 90 pass events,
  zero fail. green-run.json verifies the test-file hash equals RED. No assertions
  were changed after RED or weakened to accommodate implementation.
- First attempt failed only because /home/agent/.cache/go-build was read-only.
  environment-attempt.json/log retain this failure; it is NOT valid RED.

## Requirement-to-independent-test mapping

| PLAN rule | Tests |
| --- | --- |
| B4b.1 trusted actual context and resource ownership | TestInvalidActualContextDeniedBeforeFullAuthorization; exact-resource cases |
| B4b.2 Bootstrap, fixed owner and independent create capability | TestCreateCapabilityIsIndependent; TestTrustedBootstrapAndOwnerFullSupportedPrimitives |
| B4b.2 complete-grant OR with no deny | TestCompleteGrantUnionNeverFormsCartesianProduct; TestOnlyEffectiveSameAppMemberGroupsAndExactResources |
| B4b.2 own = immutable CreatedBy, no subordinate | TestAllOwnAndImmutableCreatedBy; unsupported-subordinate case |
| B4b.2 read/edit masks and no Cartesian expansion | union test; TestAllowedFieldsStableUniqueProjectionAndInputPurity |
| B4b.2 separate menu/data gates | TestMenuAndDataAreSeparateGates |
| B4b.2 independent task/business qualification | TestNoApprovalEligibilityOrUnknownActionFromDataEdit; no approval API exists |
| B4b.1 no cross-request policy cache | TestReplacedSnapshotsDoNotReuseEarlierAuthorization |

Trusted injection is an API boundary, not proof of source authenticity. The
core tests do not prove HTTP spoofing protection or real storage revocation.

## Verification

- Package race/coverage: exit 0, 90 pass events, zero fail; 98.5% statements.
  race-run.json, race-test.jsonl and coverage.out are the raw evidence.
- Go module vet -p 1 ./... and build -p 1 ./cmd/bff: exit 0, exact commands and
  UTC intervals in static-build-runs.json. Build target was a temporary file.
- Governance/foundation: 182 tests pass, zero fail/skip; governance.txt.
- Task checks and repository structure: exit 0; tasks.txt, structure.txt and
  repository-runs.json. These validate structure, not product acceptance.
- git diff --check passed; scope remains only the three task-owned paths.

Settings: Go1.27.1, GOTOOLCHAIN=local, GOPATH=/workspace/.weaveos-tools/gopath,
GOCACHE=/tmp/weaveos-b4b-go-cache; verification uses GOFLAGS=-mod=readonly.
No dependency/root configuration was changed. No storage service was needed.

## Reproduce

From services/bff, set the above environment to writable paths and run:

    go test -count=1 -json ./internal/apppolicy
    go test -race -count=1 ./internal/apppolicy
    go vet -p 1 ./...

For RED replay, extract red-sources.tar.gz into an isolated copy of fixed B0,
then run the first command. Replay is not the original RED execution.

## Delivery boundary

The local Git/source delivery is based on B0
5571420c8a50fe41909d75790509b9e5b6b5dbd5. Incremental Git bundle requires this
base commit; source.zip entries and the base-to-head patch allow review without
checking out a real service. The external delivery manifest records final local
HEAD and artifact hashes, avoiding a self-referential SHA in committed files.

No push, PR, merge, deployment, account grants, DB roles, HTTP or production
schema changes. PR22–25 and B4a apprefs are untouched. NOT RUN: complete BFF
runtime tests, HTTP/Session, persistent revocation/CAS, UI, Flowable/task
qualification, benchmarks or production behavior. Full-package checks are
limited to this pure core; existing product CI is not this module's validation.

Root review and separate integration authorization remain outstanding. The
local task has pr=null and stays in_progress because repository ready requires
a real PR; it is not marked accepted or deployed.
