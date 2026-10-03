# R22 RecordForm follow-up

Root test commits `01318baa9cbeb8d7812f050cd38700f561f3e3df` and `008a64ec5bdaae9ecfaf4c25e896796d1611c5e4` were integrated without editing their tests. The `008a64e` actor-isolation fixture intentionally changes actor props without a caller `key`.

The initial targeted run against `01318ba` was 13 passed / 3 failed: synchronous clean leave state at confirmation, query-context-expired refresh handling, and no same-key retry button after definite rejection. After integrating `008a64e`, the added no-remount actor test was RED (15 passed / 1 failed) before the RecordForm identity wrapper.

After the one-file RecordForm fix, editor + recovery passed 16/16 and related records/shared-operation/boundary suites passed 32/32. TypeScript check and production build passed (the build retains Vite's chunk-size advisory). Test source was not modified. Screenshots are explicitly diagnostic fixtures with test-only controls; they do not represent final UI acceptance.

Commands, raw output and exit codes are in `results/`; screenshot files and their manifest are in `screenshots/`. The run used the pinned Playwright image and offline container setup documented in each command.
