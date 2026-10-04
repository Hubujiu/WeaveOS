# R23 route baseline RED

Baseline source: `ea5b1e83eb81a9f30a05193e4a0f5a9de3d00e80` (Root test package with corrected authored specs, before R23 implementation). The first frozen Shell route case failed because the runtime form route did not render the accessible `记录工作区` region. Playwright stopped after this first valid behavioral failure (`--max-failures=1`); the other eight cases were not run. This is a partial suite RED with one captured target failure, not evidence that every case was independently observed red.

The raw mounted Playwright output, exact outer command, exit code, and error context are preserved here. Browser image: `mcr.microsoft.com/playwright@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27`.
