# Empty login password: RED → GREEN

Source: reread current v0.1.0 PRD, login rules: account and password must be nonempty. Input requirement belongs to the core login form and does not depend on Q7/Q12.

2026-09-26, pre-fix commit `5e0c90b`: `pnpm exec playwright test --config apps/web/playwright.component.config.ts --grep 'empty password'` loaded the Chromium test and exited 1 at the target feedback assertion. The empty password was submitted to the controlled API boundary; UI showed a generic 400 error rather than `请输入密码`. The test expects local feedback and zero submissions; it does not derive expected behavior from the mocked API.

Recoverable sources: `empty-password-red.spec.ts`, SHA-256 `79879F1AC8962D793E5B158B75D95EF5D7EE25F61F36D558E5729E1B166D79AC`; pre-fix App `login-before-empty-password.tsx`, SHA-256 `0D8BC783D0434B395404ADEA1781B16CB14CD944BB39B44E45C89F23A5837764`. Restore them in their original source paths for separately labeled replay.

Minimal fix: reject an empty password before setting pending/submitting. Same full component command `pnpm exec playwright test --config apps/web/playwright.component.config.ts` then exited 0, 13/13 Chromium cases passed including zero-submission assertion. Real backend E2E remains pending. No requirement/expectation was weakened.
