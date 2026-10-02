# Filter-manager cloud handoff — 2026-10-02

The user explicitly chose to move all subsequent execution to the cloud. This is a saved WIP backend stage for root review, not completion of the filter-manager feature or a claim that a cloud instance has already been rebuilt. Local development stops after this handoff is committed and the independent branch is pushed. No original PR21 branch update, merge or deployment is authorized here.

## Source and saved code

- Repository: Hubujiu/WeaveOS. Independent branch: `task/V010-020-filter-manager`.
- Baseline: original PR21 `task/V010-020-admin-shell`, `d912bb64409f807ddde79bad125b6837938f95a0`.
- Genuine independent RED commit: `ae0d103d213999f967773d7c1cbd1c2cd53d7c0c`; initial test source is recoverable from `red-tests.zip`. The subsequent WIP commit retains source, tests and actual logs, including failures.
- Approved PRD and accepted ADR008 A1–A7 are linked in `frozen-contract.md` and `contracts/table-presets.md`. Root confirmed the frozen appendix had been read back before implementation. ADR007 still governs query consistency. No new product/technical decision is made by this executor.
- Implemented backend: Session-owned members/events preset CRUD; exact trimmed Unicode naming; per-owner/view 20-slot quota; version CAS; known display columns and at least one visible business column; editable AND/OR AST; independent byte caps; registered DTO/OpenAPI/error codes; new 00005 migration and least-privilege roles; exact fixed-role/migration registration; latest backup/upgrade/audit fixtures.
- No UI implementation. No changes to published 00001–00004 migrations or query/fingerprint behavior. No active state, row results or credentials persisted in presets.

## Actual verification

| Evidence | Result and scope |
| --- | --- |
| `red-http-complete` → `green-http` | 7 real HTTP/storage tests RED → GREEN. Real PG18.6, Redis8.2.10, Go1.27.1 race, Session and application role. Includes lifecycle, isolation/auth/CSRF/revocation, exact names, quota concurrency, CAS, DTO/schema/columns/AST/Unicode/byte boundaries, unchanged revision/audit, DB constraints and least privilege. |
| `red-contract` → `green-contracts` | Missing route/DTO target RED; complete contracts 33 passed. |
| `red-role-hash`, `red-migration-approval` → `green-role-migration` | Actual old fixed-role rejection and missing exact migration registration; 2 target checks passed. |
| `red-latest-backup-fixture` → `green-backup` | Latest fixture missing new table RED; all 4 actual encrypted PG backup/restore tests passed. Complete preset config/Unicode/version/timestamps and old ledger nextval/read-only assertions retained. |
| `regression-go` | Initial full regression failed: non-loopback Docker names violate existing seed/Session safety guards, and audit cleanup omitted the new FK table. This is an environment/latest-fixture regression, not product RED or GREEN. Retained verbatim. |
| `regression-go-qualified` | Full `go test -race -count=1 -p 1 -run .* -v ./...` passed, 2026-10-02 03:30:07.200Z–03:31:16.571Z. New task Redis shares task PG network namespace, test connections are 127.0.0.1, no host ports. Existing guards unchanged; audit explicit TRUNCATE includes presets, no CASCADE or weakened assertion. 13 test packages passed, 2 packages have no tests. |
| `vet`, `build-bff` | `go vet ./...` and backend build passed. Binary is a local ignored artifact, not a release artifact. |
| `green-upgrade` | All 4 upgrade checks passed, including actual cold-first/hot-second goose upgrade through 00005, exact role policy, old data/old auth compatibility and least privilege. Fixture-owned temporary container removed by its existing finally block. |
| `regression-governance` | 132 governance/foundation checks passed. |
| `lint-openapi` | Frozen pnpm10.28.2 dependencies and Redocly2.54.2 lint passed, OpenAPI stays3.2.1. Twelve existing-area warnings (license, older conditional/unused components); no new preset warning. |
| `upgrade-invocation-error` | A mistaken test path was invoked once and failed before loading tests. Retained, not RED/GREEN; correct actual upgrade run is separate. |

JSON records contain actual commands, timestamps and exit codes; logs are preserved explicitly despite default `*.log` ignore. Test gofmt changed bytes/hash but no assertion. The 32768 canonical whole-configuration guard exists, but legal data under the other frozen caps cannot reach it; the independent bound/limitation is in `contracts/table-presets.md`. No invented 32768/32769 legal fixture or expanded capacity.

Final-head GitHub CI, Nginx HTTP/browser ingress, UI/component/visual/recording acceptance, final exact OCI build/package/promotion/restore chain and root's final acceptance remain NOT RUN for this stage. Earlier baseline CI is not evidence for this WIP. No new long test or development is started after the migration-to-cloud instruction.

## Five reference images — pixels still blocked locally

In order, authorized Library IDs and file names:

1. `libfile_abfa049d8ab88191a5ef0ba3ec044b5c` — `image(20261002-023818).png` (1170×733).
2. `libfile_6311d8e1774081918d716f4b0f9a065d` — `image(20261002-023819).png` (400×116).
3. `libfile_ccd4f521766c8191abba3b2f90c68719` — `image(20261002-023820).png` (560×674).
4. `libfile_2c3c113576ac8191bd28ab3ec5cac2ec` — `image(20261002-023821).png` (560×556).
5. `libfile_92199f676ce8819187019a859db4fc07` — `image(20261002-023822).png` (560×556).

The current Library skill and original supported helper were actually used with prepare_materialize for resolved IDs. Windows Python lacks `os.setxattr`; the helper stopped before identity-complete final materialization and the local references directory remains empty. Root-authorized native Library image_file reads returned asset pointers, dimensions and captions, with no image content/pixel bytes. None of the five was visually inspected. A read-only WSL inventory returned `Wsl/EnumerateDistros/Service/E_ACCESSDENIED`; no installation, mount or permission change was attempted. No edited helper, dropped metadata, guessed download URL, alternate credentials or imagegen.

Cloud continuation must read the current Library skill, prepare_materialize these resolved IDs to an actual cloud directory using the supported identity/version-preserving flow, verify paths exist and view all five pixels before UI. Do not assume a Windows or parent Linux path exists in the new executor.

## Preserved original-computer state

- Checkout: `D:/Data/Codex/2026-10-02/task/WeaveOS`. Original `D:/Workspace/WeaveOS` and `D:/Data/Codex/2026-10-01/task/pr21-frontend-isolated` are untouched.
- Non-secret local inventory: `D:/Data/Codex/2026-10-02/task/.test-runtime/state.json`.
- Task-only running containers: `weaveos-filter-manager-1790910244181-pg`, `weaveos-filter-manager-1790910244181-redis`, `weaveos-v010-filter-manager-loopback-redis-1790911806564`, `weaveos-v010-filter-manager-backup-1790911193923`. None publishes a host port; data retained.
- Task Go module/build and pnpm volumes retained. No old service/container/process stopped or deleted. All executor-started test/build/lint processes have completed before handoff.
- Private env files and synthetic DBs remain local and are not tracked, printed or uploaded. `.test-runtime`/`.library-runtime` are outside the repository. Ignored `.work/`, node_modules and untracked `.pnpm-store/` are generated local artifacts, not source changes. Preserve them at this handoff; do not transfer their credentials or databases.

## Non-sensitive cloud rebuilding steps

1. In root's saved WeaveOS cloud environment, fetch the independent remote branch and check out its exact confirmed remote SHA in an isolated working tree. Read AGENTS/HANDOFF/workflow/current task and applicable child rules. Recheck original PR21 head and relevant Notion sources; no integration until root says so. There are no executor sub-agents or model overrides.
2. Use repo-pinned Go1.27.1, Node24.14.0, pnpm10.28.2, PG18.6, Redis8.2.10, goose3.28.0 and browser/image versions from current workflow/config. Do not copy Windows node_modules or Go binaries. Run `pnpm install --frozen-lockfile --ignore-scripts --store-dir .work/pnpm-store` only in the cloud task tree.
3. Generate fresh random synthetic credentials in a private, untracked 0600 directory. Create fresh uniquely named task PG/Redis and test databases whose names pass the existing `_test` guards; run test clients in PG network namespace with both endpoints127.0.0.1, as the existing workflow expects. No production connection, old volume reuse or safety-guard removal. Apply `db/archive-migrations` and `db/migrations` with pinned goose, then reviewed `infra/runtime/roles.sql` to the dedicated hot test DB as owner. The fixtures use current schema; do not import the local DB.
4. Backend reproducible commands from repository root: `node --test contracts/*.test.mjs`; `pnpm exec redocly lint contracts/openapi/openapi.json`; `node --test tests/governance/*.test.mjs tests/foundation/*.test.mjs`; in `services/bff`, `go test -race -count=1 -p 1 ./...`, `go vet ./...`, `go build ./cmd/bff`, using only the new private loopback test env. For actual upgrade, build a Linux goose3.28.0 binary to ignored `.work/` and set `WEAVEOS_TEST_GOOSE` to it; precheck the fixture-owned `weaveos-v010-019-upgrade-<pid>` name has no collision before `node --test infra/server/deploy/personnel-upgrade.test.mjs`. Backup tests require a fresh own PostgreSQL container with the existing `weaveos-v010-*` name guard; see actual evidence and fixture inputs, never point at an existing service.
5. Resolve/view the five Library images, then follow frozen UI contract with independent RED first. Resume full browser/Nginx/OCI/backup/CI chain only under root's plan. Existing acceptance runner/config must be reviewed for pinned versions and unique task resources before invocation; no old cloud or local resources are implicitly reusable.

Root owns the next plan, technical decisions, implementation gate and final acceptance. The next action is cloud checkout/source/image recovery, not further coding on the original computer.
