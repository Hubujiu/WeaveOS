# Existing real acceptance scripts and the Q36 context contract

The root reported final PR21 CI run 36915063354 passing governance/Go/browser, but product run 36915063330 failing its first API stage: 123/124, missing `queryVersion` at the old personnel identity PUT. Those are root-observed remote results, not this worker's complete product acceptance. The original PR and isolated remote branch were both checked at `9574fd2bc467042ea168cf4145f6d39cc560bb7d` before this correction.

Independent sources remain the frozen Q36 PLAN §2–4 and `contracts/query-drafts.md`: guarded member identities/groups and department create/update/delete require the acting session's validated member query context; explicit refresh is page 1 POST search without an old token; object versions remain independent; simple GET compatibility remains. No contract, backend, frontend implementation, dependency or workflow is changed here.

## Reproduction and correction

`a42529e` preserves the original source and actual API RED on the retained HTTPS localhost:19445 / BFF / PostgreSQL18.6 / Redis8.2.10 stack: original two cases, 1 pass / 1 fail, the same 400 versus 200 at line 27. `8bc0a3f` adds the actual Chromium browser reproduction. Its Root-management case found zero rows and the actual page explicitly reported a changed query after the off-page API registration. The ordinary-account case in that exploratory batch failed because earlier Q36 tests had assigned the existing synthetic fixture user a non-granting identity. This is fixture reuse, not a product RED; that user's prior state was preserved.

The API script now explicitly obtains the current acting session's POST member search before each guarded domain write. After each write the next operation starts a fresh page-1 baseline. No failed business write is retried. Original non-Root grant, template-source, invitations, Root-only password reset, live revoke, object-version conflict, department move/access invariance, audit secrecy, simple GET reads and cleanup assertions remain. Member/root-department conflicts now also assert `PERSONNEL_CONFLICT`, so a stale-context 409 cannot satisfy them. Department rename and stale-version checks exercise the other mandatory write input.

Permission-denied requests retain expected 403; five guarded write families additionally use structurally valid object input and an unknown, nonempty context to prove permission denial precedes context lookup. Added live-revoked write checks use the same principle. An initial exploratory assertion expected 403 for missing required fields (and then invalid department version zero). Those exploratory requests violate existing request structure; their 400 is not proof of an authorization production defect. `missing-field-permission-probe.txt` is preserved transparently, not counted as the approved script RED and not used to justify changing the backend. The final requests respect department version >=1 while deliberately using an unknown context.

The real Root browser case explicitly clicks Refresh Query after the background registration and before its search. It asserts POST search 200 and that the actual identity PUT sends the returned search context and independent original object version zero, then receives 200. All existing shared template/identity UI, account sources, non-Root invitation, template live revoke and direct-admin denial assertions remain. No mock API, screenshots, traces, videos or credentials are persisted by these credential-bearing cases.

## Local verification and fixture provenance

Private fresh fixtures are created through the existing real API in the retained isolated stack: new ordinary/reset accounts and new independent invitation codes; original fixtures and drafts are untouched. They remain ignored under `.work/`. No other instance/process/worktree was modified or stopped.

The first full API batch's added overstrict permission probes gave 122/124; its complete log is the probe evidence above. After respecting structural validation, one repeat gave 123/124 because the original FR-002 fixed 254-character synthetic account had already been created by that first batch. `full-api-retained-data-attempt.txt` records that environment collision. Only that worker-created fixed test account was renamed (exactly one affected row), preserving its data; fresh private invitations and accounts were prepared again. Neither that original test nor its 201 assertion was changed.

The final complete uninterrupted `node --test tests/acceptance/api.test.mjs` batch passed **124/124**, zero skips (`full-api-green.txt`). It includes both current personnel cases plus all original independent expanded/API cases. The related actual personnel browser command passed **6/6** across Chromium, Firefox and WebKit (`personnel-browser-green.txt`), zero retries/skips. Root `pnpm typecheck`, including `tsconfig.tests.json`, and 132 governance/foundation checks pass. No unchanged complete component/source build/Go suite was rerun for test-only changes; the prior remote source CI is attributed to 9574fd2.

```powershell
$env:WEAVEOS_API_URL='https://localhost:19445'
$env:WEAVEOS_WEB_URL=$env:WEAVEOS_API_URL
$env:WEAVEOS_ACCEPTANCE_FIXTURES='<private fresh synthetic fixtures.json>'
$env:NODE_EXTRA_CA_CERTS='<isolated cert.pem>'
node --test tests/acceptance/api.test.mjs
node node_modules/@playwright/test/cli.js test --config apps/web/playwright.integration.config.ts --grep 'R2/Q25 real|R3 real Root' --output .work/q36-personnel-browser-green
```

## Remaining runtime bridge (not applied)

A global call-site review of tests and runtime runner found the same three missing guarded inputs in `infra/runtime/operations.test.mjs:53–56`: identity PUT, department POST, member groups POST. Both product and runtime pipelines execute this file later. The exact minimal reviewable patch is `runtime-context.patch`, checked with `git apply --check`; it adds an explicit fresh member POST search before each of those writes, preserving all backup/restore/rollback/revocation/audit assertions. It has not been applied, and a complete runtime recovery/rollback batch has not run locally.

That file is outside the current task metadata's registered allowedPaths. Root AGENTS §5 requires “只改任务允许路径；共享文件先登记协调，不覆盖他人修改，不盲目git add -A。” The root owns integration and scope; this worker requested the specific file extension and presents the patch for root handling. Until that is resolved, no further PR push is made to knowingly trigger a pipeline with the remaining missing inputs. Root owns the final head's remote CI/Notion and acceptance. No main merge/deployment or permission/contract relaxation occurred.
