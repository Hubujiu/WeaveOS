#!/usr/bin/env bash
set -uo pipefail
docker run --rm --init \
  --mount type=bind,src=/tmp/V012-R20-source-20261003T191958Z-852594,dst=/repo \
  --mount type=bind,src=/tmp/V012-R20-bin-root-spec,dst=/r20-bin,readonly \
  -e PATH=/r20-bin:/repo/apps/web/node_modules/.bin:/repo/node_modules/.bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
  -w /repo \
  mcr.microsoft.com/playwright@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27 \
  bash -euc 'pnpm exec tsc -p tsconfig.tests.json && (cd apps/web && pnpm exec tsc --noEmit)' \
  2>&1 | tee /tmp/V012-R21-followup-typecheck/typecheck.log
status=${PIPESTATUS[0]}
printf '%s\n' "$status"
exit "$status"
