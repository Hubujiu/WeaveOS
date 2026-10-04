#!/usr/bin/env bash
set -euo pipefail
repo=/workspace/WeaveOS-worktrees/V030-012-R26
log="$repo/docs/evidence/V030-012/root-r26/checks/typecheck-build.log"
set +e
docker run --rm --init --shm-size=1g \
  --mount type=bind,src="$repo",dst=/repo \
  --mount type=bind,src=/tmp/V012-R23-followup-deps/apps/web/node_modules,dst=/repo/apps/web/node_modules \
  --mount type=bind,src=/tmp/V012-R23-followup-deps/node_modules,dst=/repo/node_modules \
  --mount type=bind,src=/tmp/V012-R20-bin-root-spec,dst=/r20-bin,readonly \
  -e PATH=/r20-bin:/repo/apps/web/node_modules/.bin:/repo/node_modules/.bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
  -w /repo \
  mcr.microsoft.com/playwright@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27 \
  bash -euc 'pnpm exec tsc -p tsconfig.tests.json && cd apps/web && pnpm exec tsc --noEmit && pnpm exec vite build' \
  > "$log" 2>&1
status=$?
cat "$log"
printf '\nEXIT=%s\nLOG=%s\n' "$status" "$log"
exit "$status"
