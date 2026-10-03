# Root R24 focused real-record acceptance

Root personally authored this runner, configuration and the full journey appended to tests/acceptance/applications-web.spec.ts. Do not edit any of them as an executor; report failures to Root.

## Exact prerequisites
- Use a fresh isolated worktree at the final Root-approved frontend commit, merged with backend 6ab4ec923cb05f9ff5944064db67007df5bf1524. Root will provide the final frontend SHA.
- Do not use the old original dirty working trees. Do not overwrite an existing .work/root-record-acceptance directory or reuse unknown private fixtures.
- Existing Docker/Compose and pinned official images are required. The Go container is golang:1.27.1, matching .go-version, with GOTOOLCHAIN=local. It verifies the toolchain version before building. No host Go binary, trust-store changes or certificate bypass are used.
- Only then run: node docs/evidence/V030-012/root-r24/run-focused.mjs

The runner creates an isolated PostgreSQL/Redis/BFF/Nginx stack from existing infra/acceptance/compose.json, uses same-origin HTTPS, seeds isolated test accounts through the existing seed command, builds the actual frontend and backend, and executes exactly one Chromium test. No API route mocking or application data seeding substitutes for the UI journey. The workflow creates an application, creates and saves a configured form, creates a normalized record, reads it, edits it, and verifies persistence after browser reload.

## Evidence and scope
Host-mounted .work/root-record-acceptance/public contains complete outer command manifests, stdout/stderr logs, exit codes, Playwright JSON and non-login screenshots. Private env, fixture and certificate/key files remain outside public evidence. Known generated secrets are redacted from public logs/report. Review the public files before publishing.

The script stops only its own Compose project, preserving its isolated volume/fixtures for diagnosis. It never deploys or touches main. A retry requires Root's inspection of the failed stage and a fresh worktree/directory decision; never delete prior evidence just to rerun.

This is a focused real journey, not the full product acceptance gate, Firefox/WebKit coverage, capacity benchmark, workflow/approval acceptance, or complete v0.3 delivery. Canonical acceptance scripts and CI gates remain unchanged. Root ran Node syntax checks locally; the real runner has not yet been executed.


## First diagnostic and harness correction

The provisional frontend d147c7e plus backend6ab4ec merged cleanly in23bfcebe49bfe790bd656f20991582023e3ff48c. The original runner stopped at stage005: the older Go1.25.7 container could not verify proxy.golang.org while auto-fetching Go1.26. No browser test ran; its private directory and sanitized public failure evidence remain preserved.

The initial host-Go preparation claim was not verified: /usr/bin/go was later probed and is not the Go compiler. Root did not release or use the contemplated host-Go runner. The official golang:1.27.1 image was actually checked with network disabled and reports go1.27.1 linux/amd64, exit0. Root updated this container-based runner to that repository-pinned version with GOTOOLCHAIN=local, retaining normal TLS verification. Root syntax-checked the correction; real execution is still pending. Retry only in a new isolated worktree, never over the failed evidence.
