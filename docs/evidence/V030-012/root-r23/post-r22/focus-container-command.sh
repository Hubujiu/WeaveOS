#!/bin/sh
# Focus restoration regression after the R23 RecordRoute fix.
docker run --rm \
  -v /tmp/V012-R20-source-20261003T191958Z-852594:/repo \
  -v /tmp/V012-R20-bin-root-spec:/r20-bin:ro \
  -v "$PWD/docs/evidence/V030-012/root-r23/post-r22/focus-output:/out" \
  -w /repo/apps/web \
  -e PATH=/r20-bin:/repo/apps/web/node_modules/.bin:/repo/node_modules/.bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
  mcr.microsoft.com/playwright@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27 \
  sh -lc 'pnpm exec playwright test --config playwright.component.config.ts --output=/out --project=chromium src/root-record-shell.component.spec.ts --grep "dirty record modal close"'
