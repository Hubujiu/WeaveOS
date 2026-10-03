# R21 synchronous confirmation follow-up

The published baseline was `0ed9128`; Root's frozen test package is `36ea81a` (parent `0ed9128`). On that baseline the newly strengthened `confirmed()` assertion observed `write_in_flight` synchronously after the valid receipt had already been accepted. The exact focused run failed one assertion (13 passed, 1 failed). The hook now clears its synchronous in-flight flag and retires that lifecycle before calling `confirmed`, so a consumer may safely initiate a new operation inside the callback without the old operation's `finally` clobbering it.

The focused operation/scoped/leave/boundary component suite passed 29/29 after the change. The initial typecheck attempt was blocked by syntax errors in Root's frozen `applications.form-shell.component.spec.ts` at lines 47, 287, and 519 (unterminated string literals and follow-on parse errors). Those tests are Root-authored and were not edited. The corrected-package typecheck result is recorded below.

Raw logs and exit codes are preserved here. Browser runs used the pinned Playwright image `mcr.microsoft.com/playwright@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27` and Root's existing installed dependencies; the disposable dependency source was outside the repository.

After Root corrected the migrated test source in `ea5b1e8`, the repository tests TypeScript project and web application TypeScript project were rerun against that clean baseline containing the R21 fix. Both passed with exit code 0. Their raw output (empty on success), command, and exit code are in `typecheck-corrected-*`. The earlier syntax failure remains preserved as the initial blocked attempt.
