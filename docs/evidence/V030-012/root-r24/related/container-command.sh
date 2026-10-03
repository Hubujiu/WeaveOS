#!/bin/sh
docker run --rm \
  -v "$PWD:/repo" \
  -v /tmp/V012-R23-followup-deps/apps/web/node_modules:/repo/apps/web/node_modules \
  -v /tmp/V012-R23-followup-deps/node_modules:/repo/node_modules \
  -v /tmp/V012-R20-bin-root-spec:/r20-bin:ro \
  -v "$PWD/docs/evidence/V030-012/root-r24/related/output:/out" \
  -w /repo/apps/web \
  -e PATH=/r20-bin:/repo/apps/web/node_modules/.bin:/repo/node_modules/.bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
  mcr.microsoft.com/playwright@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27 \
  sh -lc 'pnpm exec playwright test --config playwright.component.config.ts --output=/out --project=chromium src/applications.form-shell.component.spec.ts src/applications/root-operation-guard.component.spec.ts src/applications/root-scoped.component.spec.ts src/applications/root-boundary.component.spec.ts src/applications/root-record-leave.component.spec.ts src/applications/root-record-read-contract.component.spec.ts src/applications/root-record-workspace.component.spec.ts src/applications/records/record-form.component.spec.ts src/applications/records/records-panel.component.spec.ts src/applications/records/records.component.spec.ts src/applications/records/root-recovery.component.spec.ts src/applications/records/root-editor.component.spec.ts'
