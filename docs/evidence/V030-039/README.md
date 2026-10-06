# V030-039 independent dependency security evidence

Source: approved task PRD/ADR and GHSA-68fv-2mgg-jv7q (linked in task document).
User approved current dependency and rollback candidate repair, not deployment/main merge.

## Actual RED and GREEN
Root personally ran `pnpm audit --json` using pinned pnpm10.28.2 before modification against current479b416 and original rollback85c2ee; both exit1, exactly source-map-js high-severity advisory. Original JSON and exit codes are preserved. These are real advisory failures, not setup failures.
`pnpm -r update source-map-js --depth 100 --lockfile-only --ignore-scripts` changed only source-map-js version/integrity/references. Current and original-rollback-derived candidate each then had audit exit0/all counts0. Frozen installation with scripts disabled, project typecheck and build each passed for both source sets. Local Node24.19.0 is explicitly different from CI's pinned24.14.0; CI must still verify.

Original source-map-js1.2.1 to patched1.2.2 integrity comes from official npm metadata saved here, verified by frozen installation. Current lock diff is five changed lines, old rollback four; no other package upgrade, direct manifest or user code change. Exact candidate tree independently matched GitHub-created tree.

## Boundaries
Original rollback85c2ee/tag not rewritten. Candidate c372267 is retained by evidence/V030-039-rollback-patched and changes only pnpm-lock.yaml relative to old source. Current runtime fixture can select that exact candidate for real validation; this is not a production rollback or deployment.
Initial local tool cache setup tried absent /home/agent paths and failed before changes; rerun used owned workspace caches. Not counted as RED.
Actual container runtime/restore/rollback remains NOT RUN locally; CI results must be added separately. Previous V037 WebKit failure is not fixed by this package patch.
