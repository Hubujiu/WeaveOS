#!/usr/bin/env bash
set -euo pipefail
# Set TEST_FILES to the requested frozen Playwright file list. Pin and mounts
# match the run used for red.log and green.log.
docker run --rm --init --shm-size=1g \
  --mount type=bind,src=/tmp/V012-R20-source-20261003T191958Z-852594,dst=/repo \
  --mount type=bind,src=/tmp/V012-R20-bin-root-spec,dst=/r20-bin,readonly \
  --mount type=bind,src=/tmp/V012-R21-R23-green,dst=/artifacts \
  -e CI=true -e PLAYWRIGHT_BROWSERS_PATH=/ms-playwright \
  -e PATH=/r20-bin:/repo/apps/web/node_modules/.bin:/repo/node_modules/.bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
  -w /repo/apps/web \
  mcr.microsoft.com/playwright@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27 \
  bash -euc 'pnpm exec playwright test --config playwright.component.config.ts --output=/artifacts/operation src/applications/root-operation-guard.component.spec.ts src/applications/root-scoped.component.spec.ts src/applications/root-boundary.component.spec.ts src/applications/root-record-leave.component.spec.ts'
