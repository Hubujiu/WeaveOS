# Root R25 real-CI repair plan

Base: a5897483365e5f6c467a25cf834543fff3b04fc9. Actual final CI37161951003 job111317050273:126/138 browser cases pass,12fail. No claim of full acceptance.
Authoritative detail: V030-012 ADR §17/17.1.

## Inputs and exact implementation boundary
Root personally authored applications-web.spec.ts isolation repair and two topology.test.mjs cases. Do not edit these tests.
Only implementation files:
1. infra/acceptance/run.mjs
2. infra/runtime/run.mjs
In each existing private runtime.env template append exactly the four existing BFF settings:
- WEAVEOS_DEFINITION_HMAC_KEY: independent randomBytes(32).toString('base64'), not the audit key
- WEAVEOS_DEFINITION_KEY_ID=test
- WEAVEOS_SCHEMA_LOCK_TIMEOUT_MS=1000
- WEAVEOS_SCHEMA_STATEMENT_TIMEOUT_MS=5000
Keep the existing private file writer, permissions, key generation ownership, deployment role/network/TLS configuration and all pipeline steps. Do not introduce production defaults, weaken missing-config behavior or change Go service code. The runtime simulation is isolated CI, not deployment.

## Root test corrections
Permission-mutating application tests register fresh members through actual invitation/registration endpoints. The baseline unprivileged user remains untouched and the password-reset target remains exclusively owned by password-reset tests. Keep every existing ownership/grant/deny/empty-home assertion and all138 browser cases. The root journey remains120s with no per-assertion timeout changes.

The topology regression captures actual runAcceptance runtime.env using its existing execute injection, deliberately stopping at the first external OpenSSL command before Docker. It checks dedicated canonical32-byte key, independence from audit and across runs, exact budgets and0600. The second checks immutable-runtime simulation wiring. Never publish generated private env files or secret bytes.

## Execution
1. Preserve current WIP/evidence. Fetch this Root commit into the frontend task branch worktree.
2. Run node --test tests/acceptance/topology.test.mjs before implementation. Preserve complete RED stdout/stderr and exit code.
3. Apply only the two environment insertions. Run the same tests GREEN. Run repo/web TypeScript, task/repo structural checks, diff whitespace, node --check for both runners and actual canonical Playwright --list. No browser success claim from list.
4. Preserve exact sanitized logs in root-r25; force-add exact .log paths because they are ignored. Verify manifest files exist in committed tree.
5. Commit and push only task/V030-012-original-app-shell, retaining Root test ancestry. No main or deploy. Root reviews exact diff and final-head CI. Do not run the blocked local stack or change CA trust.
6. If tests expose a further mismatch, report exact evidence to Root; do not alter tests, widen configuration scope or add retries to hide it.

Time/space: constant-size environment generation and validation; per-case fixture creation uses a constant number of actual API calls and separate synthetic accounts. No full-data copy or production-data modification.

Earlier same-source WebKit WEB17 session expiry failed once but passed final a589. No proof of Redis resurrection; preserve evidence without weakening or blind retries.
