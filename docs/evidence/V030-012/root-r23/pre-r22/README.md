# Root R23 Shell integration before RecordForm R22

Source base: `ea5b1e83eb81a9f30a05193e4a0f5a9de3d00e80` with the R23 route implementation under test and the original `RecordForm` save/recovery interface. The pinned Chromium shell suite ran all nine Root-authored cases: 3 passed and 6 failed. The ordinary runtime route, direct ordinary-user design denial, and runtime-origin return passed; editor operations, leave handling, candidate search, and stale-query opening were not yet supported by the old editor interface or by the strict-mode fixture setup.

The pre-R22 typecheck exited 2 because the route intentionally supplies `queryVersion`, `registerLeaveGuard`, and the other reviewed R22 props while the old `RecordForm` does not expose them. The Vite build passed. This is dependency evidence, not the final R23 result.

Commands and raw shell output are in this directory. Playwright ran in `mcr.microsoft.com/playwright@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27`.
