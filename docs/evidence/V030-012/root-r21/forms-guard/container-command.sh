set -o pipefail
docker run --rm --init --shm-size=1g \
  --mount type=bind,src=/tmp/V012-R20-source-20261003T191958Z-852594,dst=/repo \
  --mount type=bind,src=/tmp/V012-R20-bin-root-spec/pnpm,dst=/r21-bin/pnpm,readonly \
  --mount type=bind,src=/tmp/V012-R21-resume,dst=/artifacts \
  -e CI=true -e PLAYWRIGHT_BROWSERS_PATH=/ms-playwright \
  -e PATH=/r21-bin:/repo/apps/web/node_modules/.bin:/repo/node_modules/.bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
  -w /repo/apps/web \
  mcr.microsoft.com/playwright@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27 \
  bash -euc 'pnpm exec playwright test --config playwright.component.config.ts --output=/artifacts/forms-output src/applications/forms/forms.component.spec.ts' \
  2>&1 | tee /tmp/V012-R21-resume/forms.log
