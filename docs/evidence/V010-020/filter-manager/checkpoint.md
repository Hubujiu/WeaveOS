# Filter-manager first implementation checkpoint

## Resumed backend result and cloud handoff

The earlier checkpoint below is historical. Root authorized the explicit audit
TRUNCATE latest-schema fixture and loopback-qualified task connections; complete
Go race, vet/build, actual cold-first/hot-second upgrade, 132 governance checks
and pinned OpenAPI lint now pass. Existing safety guards/assertions remain.
The user's subsequent explicit decision moves all further execution to cloud;
local execution stops after saving and normally pushing the WIP backend stage.
See [cloud-handoff.md](cloud-handoff.md) for exact evidence, unverified work,
non-sensitive rebuild instructions, preserved containers and image-pixel blocker.
No UI, final-head CI, Nginx or exact final OCI release claim is made.

## Earlier first-stage checkpoint

Workspace: `D:/Data/Codex/2026-10-02/task/WeaveOS`.
Branch: `task/V010-020-filter-manager`; baseline d912bb64409f807ddde79bad125b6837938f95a0.
RED commit ae0d103 pushed to the new branch. Implementation is saved but uncommitted.
Root requested this checkpoint and continuation; no further long tests started.

Sources actually read: project → PRD entry → current approved PRD → Architecture → accepted ADR008 A1–A7; accepted ADR007 query/filter/time semantics; ADR002; root/child AGENTS, HANDOFF, workflow, task index/current record, testing. No source approvals were edited, no delegation/model override requested, no merge/deploy/original branch update.

Completed independent RED → GREEN:

- HTTP/DB: 7 target tests failed on missing route/table, then passed with actual Session, PG18.6, Redis8.2.10, Go1.27.1 race and SET ROLE auth_app. Lifecycle, exact trim/Unicode naming, owner/view, revoked permission/CSRF, 40 concurrent creates constrained to 20 slots, CAS, closed DTO/known columns/at least one visible, editable AST, independent filter/raw byte boundaries, no revision/audit effects, database constraints and least privilege.
- Contract: two initial target failures (missing route/DTO), then 33/33 complete contracts passed.
- Role policy/compatibility: actual old fixed-hash rejection and missing exact migration approval observed, then 2/2 target tests passed with literal approved roles hash and exact 00005 SHA.
- Backup fixture: actual old latest fixture failed on missing presets; 4/4 encrypted backup tests passed after adding 00005 and an independent synthetic saved configuration. Full config fields/Unicode/CAS/timestamps survive; ledger sequence nextval=3 and backup cannot advance it remain asserted.

Full Go regression was run once (`regression-go.json`/`.log`), exit 1. This is not GREEN or a product RED: seed and Session require local/loopback targets, while the initial dedicated test env used Docker DNS names. Audit maintenance's explicit TRUNCATE list omits the new FK table; update only its latest fixture list, not CASCADE/guards. Raw full output is retained. Other completed tests must not be advertised as replacing this unresolved full regression.

Images: original supported helper failed at Windows os.setxattr, no identity-complete materialization. Root then authorized native Library read of all five image_file references. The tool returned structured content_type=image_asset_pointer entries, dimensions and captions, with no image content blocks/pixel bytes. None has been visually inspected by the executor; UI and screenshot/video work remains paused. No alternate download, edited helper, discarded metadata or imagegen used.

Remaining: loopback-qualified synthetic test layout, audit latest-schema TRUNCATE fixture, full Go/vet, actual full upgrade/packaging and pinned OpenAPI lint. OpenAPI's current JSON rewrite also needs formatting-only diff reduction. The 32768 canonical configuration check exists, but a legal boundary fixture cannot reach it under the other frozen caps; see contracts/table-presets.md for the independent bound and root review note.

No UI changes or Vite service. No final-head CI/Nginx/new UI regressions/screenshots/recordings yet. All invoked test processes have completed; task-only PG/Redis/backup containers are retained. Their non-secret inventory and private env files are outside the checkout in `.test-runtime/`; preserve them, never print credentials. No old containers/worktrees/services were mutated.

Evidence files use actual starts/completions/exit codes and source hashes. Initial unformatted tests are recoverable in red-tests.zip. Formatting changed the current test hash, not its assertions. Raw .log files are excluded by the repository default ignore and are explicitly staged for preservation in the eventual implementation commit.
