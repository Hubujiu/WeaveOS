#!/usr/bin/env bash
set -euo pipefail
source_root=/tmp/V012-R17-source-20261003T182915Z-780871
shim=/tmp/V012-R17-bin-static
log=/tmp/V012-R17-static.log
mkdir -m 700 "$shim"
cat > "$shim/pnpm" <<'SHIM'
#!/bin/sh
case "$1" in
  exec) shift; exec "$@" ;;
  typecheck) exec tsc --noEmit ;;
  build) exec vite build ;;
  *) printf 'R17 pnpm shim unsupported command: %s\n' "$1" >&2; exit 2 ;;
esac
SHIM
chmod 700 "$shim/pnpm"
docker run --rm --init --shm-size=1g \
  --mount type=bind,src="$source_root",dst=/repo \
  --mount type=bind,src="$shim",dst=/r17-bin,readonly \
  -e CI=true -e PLAYWRIGHT_BROWSERS_PATH=/ms-playwright \
  -e PATH=/r17-bin:/repo/apps/web/node_modules/.bin:/repo/node_modules/.bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
  -w /repo/apps/web \
  mcr.microsoft.com/playwright@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27 \
  bash -euc 'pnpm typecheck && pnpm build' > "$log" 2>&1
status=$?
cat "$log"
printf '\nEXIT=%s\nLOG=%s\n' "$status" "$log"
exit "$status"
