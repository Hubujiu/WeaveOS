docker run --rm --init --shm-size=1g \
  --mount type=bind,src=/tmp/V012-R20-source-20261003T191958Z-852594,dst=/repo \
  --mount type=bind,src=/tmp/V012-R20-bin-root-spec/pnpm,dst=/r20-bin/pnpm,readonly \
  --mount type=bind,src=/tmp/V012-R20BC-artifacts-green,dst=/artifacts \
  -e CI=true -e PLAYWRIGHT_BROWSERS_PATH=/ms-playwright \
  -e PATH=/r20-bin:/repo/apps/web/node_modules/.bin:/repo/node_modules/.bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
  -w /repo/apps/web \
  mcr.microsoft.com/playwright@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27 \
  bash -euc 'pnpm exec playwright test --config playwright.component.config.ts --output=/artifacts/related-output src/applications/root-boundary.component.spec.ts src/applications/root-scoped.component.spec.ts src/applications.component.spec.ts src/applications.form-shell.component.spec.ts src/applications.permissions.component.spec.ts src/applications.states.component.spec.ts src/applications/forms/forms.component.spec.ts src/applications/records/record-form.component.spec.ts src/applications/records/records-panel.component.spec.ts src/applications/records/records.component.spec.ts src/applications/records/root-recovery.component.spec.ts' \
  2>&1 | tee /tmp/V012-R20BC-artifacts-green/related.log
