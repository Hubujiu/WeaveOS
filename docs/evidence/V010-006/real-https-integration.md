# Real HTTPS frontend integration

2026-09-26, final app code includes Q7/Q12/Q13. Backend task005 accepted on remote main7019a2c (PR9, verified final head1d3f233, CI36216068873). Frontend23 Chromium component tests, typecheck/build pass. These do not replace product tests.

## Real boundaries and commands

Local WSL Docker Desktop Linux29.6.2: PostgreSQL18.0, Redis8.2.1, compiled real Go BFF; fresh isolated weaveos_web_acceptance database migrated with Goose3.28.0, synthetic fixture produced by existing acceptance-seed. Private fixture/config/test-key files stay outside Git. Real same-origin HTTPS test bridge serves the actual built frontend and forwards to BFF; Secure/HttpOnly/Host prefix retained. Self-signed local test certificate is accepted by browser test contexts; Node HTTP acceptance explicitly trusts its certificate. This fixture is a test harness, not production hosting or certificate approval.

`WEAVEOS_API_URL=https://localhost:9443 NODE_EXTRA_CA_CERTS=<local-test-cert> WEAVEOS_ACCEPTANCE_FIXTURES=<private-file> node --test tests/acceptance/api.test.mjs`

Actual 17/17 HTTP tests pass (exit0), including registration/login/current/logout, concurrency, duplicate-account rollback, Bootstrap/reset, disabled login and CSRF rejection. Readiness uses real dependencies.

`pnpm exec playwright test --config apps/web/playwright.integration.config.ts`

Initial Windows run:29/30 pass; WebKit context reports SameSite=None where approved expectation is Lax. Kept exact Lax checks. No application/Cookie change, no assertion weakening. Diagnostic Linux WebKit run of the same login/reload/logout case:1/1 pass. Then three fresh UI invitations were generated through the real Bootstrap endpoint and all tests ran in Linux Docker:

`docker exec -w /repo <isolated-playwright-container> pnpm exec playwright test --config apps/web/playwright.integration.config.ts`

Image `mcr.microsoft.com/playwright:v1.63.0-noble@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27`. Actual30/30 pass (1.2m), Chromium/Firefox/WebKit; exact host Cookie/CSRF attributes, header binding, no frontend credential storage, real refresh restore and logout revoke. No API mocks. Windows observation remains recorded; its third-party cause is not claimed as proven. Linux is the user-authorized target simulation.

The old registration test omitted confirmation; corrected the fixture interaction from PRD/Figma40:2, retaining its registration/no-session expectations. New Cookie/CSRF assertions verify existing behavior directly GREEN; no new business implementation or invented RED.

## Manual handoff and limits

Visual target: WaveOS Login13:2/Register40:2. Curated desktop/mobile screenshots and mapping are in new-prototype.md, waveos-login-final.png, waveos-register.png, waveos-login-mobile-after-stack.png. User is the final visual approver (Q10); this task delivers the review material and does not sign MAN-UI. Full007 contract/security/fault coverage and reproducible Linux Nginx test stack,008 archive/recovery/rollback and complete release gate still remain. No production deployment.
