# Cloud checkpoint — 2026-10-02

Root confirmed that original-computer coding stopped and handed over
`5c2d87a52217fe3853c785d8e7a9620a6ede12c4`. This executor fetched and fast-forwarded
the independent `task/V010-020-filter-manager` checkout to that exact SHA.
Actual cwd is `/workspace/WeaveOS-worktrees/V010-020`. Original PR21 remains
`d912bb64409f807ddde79bad125b6837938f95a0`. No original PR branch update, merge,
deployment, real-server role installation or real-data restore occurred.

## Sources and scope

Root/child AGENTS, HANDOFF, workflow, testing, current task and complete
`cloud-handoff.md` were read. `/workspace/.agents` and `/workspace/.codex` are
empty; no repository `.agents` skill directory exists. The current Library
skill, resolved-reference materialization reference and its unchanged current
transfer helper were obtained from the current skill source.

Notion project, PRD and Architecture entry pages were fetched. The approved
[PRD](https://app.notion.com/p/3ed2f5a9e6488156bf31f129d467c340), accepted
[ADR008](https://app.notion.com/p/3ed2f5a9e64881639ba2c434fb6c7c34), both complete
A1–A7 appendices, and the applicable current
[ADR007](https://app.notion.com/p/3ec2f5a9e64881c19aa8d5fc6d821ab8) rules were read.
Last-edited timestamps: 2026-10-02T02:58:47.868Z, 2026-10-02T02:58:52.013Z and
2026-10-01T14:19:40.045Z, respectively. The false PRD archive flag does not cancel
its approved scope. No Notion/Figma content or state was changed.

Existing DTOs, strict parsing, CRUD, schema, quota/CAS, editable AST, references,
column constraints and exact role/migration registration were reviewed. No
production implementation was rewritten. The independent 32KiB whole-content
guard is retained; otherwise legal current DTOs cannot reach it under the other
frozen bounds. No invented legal 32768/32769 fixture or limit expansion was added.
Apply-time current field/reference validation is still a pending UI requirement.

## Five Library images: actual network blocker

Both preparations returned authorized transfers for all five exact Library IDs,
zero unavailable items, `workspace_path=null` and signed download transfers to
the explicit actual executor destination
`/workspace/weaveos-cloud-prep/library-references`. The initial original-helper
attempt and one supported retry failed for every file with exit 1:

`library file transfer failed: download failed`

After root explicitly requested underlying diagnostics, exception observation
around the unchanged helper identified the actual failure:

`URLError -> OSError: Tunnel connection failed: 403 Forbidden`

This is a network proxy CONNECT-tunnel rejection before the file endpoint's
download response, local installation or xattr writes. It is not a nonexistent
returned workspace path. Linux `os.setxattr` exists; this is not the old Windows
missing-xattr failure. The proxy's particular access rule was not supplied.
Exact per-file initial/retry/diagnostic results are in
`library-reference-status.json`; signed URLs and authorization headers are not
included. No root-environment path is assumed shared with this executor.

| Order | File | Actual pixels |
| --- | --- | --- |
| 1 | image(20261002-023818).png | BLOCKED; no readable local PNG |
| 2 | image(20261002-023819).png | BLOCKED; no readable local PNG |
| 3 | image(20261002-023820).png | BLOCKED; no readable local PNG |
| 4 | image(20261002-023821).png | BLOCKED; no readable local PNG |
| 5 | image(20261002-023822).png | BLOCKED; no readable local PNG |

Zero images were viewed. No alternate download route, helper patch, omitted
identity metadata, guessed URL, raw-pointer read, TLS verification bypass or
image generation was used. Current resolved-reference materialization returned
no direct local-path alternative for this executor. List/search download uses
the same preparation/transfer route. Root must resolve this executor's supported
Library transfer access before UI implementation; text descriptions do not
replace reference pixels.

## Actual fresh cloud verification

New task-only PG18.6/Redis8.2.10, random synthetic credentials and isolated
`_test` databases were created. No original computer database, credentials,
binary, node_modules or volume was transferred. Test clients share the new PG
network namespace and use 127.0.0.1 and Redis DB15. No host ports are published.
Private 0600 files and inventory are outside the repository at
`/workspace/weaveos-cloud-prep/private/state.json`.

`/usr/bin/go` is the desktop program whose help says:
`Go - Local two-player Go on a 13x13 board.` The real compiler used was the
repository-pinned Go1.27.1 Docker image. Node24.14.0, pnpm10.28.2, goose3.28.0
and exact pinned PG/Redis images were used. The existing executor CA was mounted
read-only into install containers, retaining TLS verification. Pinned Node was
copied from its image for host checks requiring OpenSSL and Docker.

| Check | Cloud outcome |
| --- | --- |
| Fresh cold then hot goose migrations | PASS through 00005; 00001–00004 unchanged |
| Reviewed roles in new synthetic hot DB | PASS; no real-server permissions configured |
| Full Go race `-count=1 -p 1 -json ./...` | 300 test/subtest PASS records; 12 packages PASS, 2 packages have no tests; no test skips/failures |
| Go vet / BFF build | PASS; binary is an ignored local artifact |
| Contracts + foundation/governance, pinned host Node | 165 PASS, 0 failures/skips |
| Frozen pnpm / typecheck / web build | PASS; existing chunk-size warning remains |
| Redocly2.54.2 / OpenAPI3.2.1 | PASS; 12 existing-area warnings |
| Actual encrypted backup/restore | 4 PASS; complete preset fields and original ledger/read-only assertions preserved |
| Actual upgrade / exact fixed roles | 4 PASS; 00005, old-auth/data compatibility and least privilege preserved |
| Repository/task structural checks / whitespace | PASS; structural checks do not establish product, source or TDD acceptance |

Complete Go events, commands/timestamps/exit codes, log hashes and
`full-go-summary.json` are retained here. Original genuine RED remains
`ae0d103`/`red-tests.zip`; this cloud replay is a regression, not new RED.
First dependency TLS failure and the Node slim-container OpenSSL failure are
retained separately from successful corrected runs. Neither is product RED or
GREEN. Original/corrected pnpm logs are in `frozen-install-logs.zip`. No assertion,
timeout, skip, expected result or pinned dependency was weakened.

## Remaining work and next action

Manager/editor, AND/OR UI, persistence client, controlled visibility, active
snapshot/baseline, component/real-browser acceptance, screenshots and
Chromium/Firefox recordings remain NOT IMPLEMENTED / NOT RUN. Actual Nginx,
final OCI/package/promotion/restore and final-head CI are NOT RUN. Backend
regression does not substitute for those checks.

This commit changes only cloud evidence and task progress: TDD:N/A for the
documentation-only diff. The pinned archive Gitleaks check runs against the
checkpoint commit after creation; its exact head/result is reported in the
handoff, not conflated with the original minimal scan or a complete final
security suite. Root owns the next plan and acceptance. Resume UI after supported
pixel access is resolved; keep all execution in this saved cloud and independent
branch. Task containers, ignored dependencies/tools and private state are
retained. All executor-started test/install processes have completed.
