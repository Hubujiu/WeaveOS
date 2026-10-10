#!/bin/bash
set -euo pipefail
ROOT=/workspace/scratch/75c2273f6a80
TASK=V030-068; LABEL=$1; MODE=$2
[[ "$LABEL" =~ ^[a-z0-9-]+$ ]] || exit 2
R=$ROOT/WeaveOS-worktrees/$TASK
FIX=$ROOT/app-recovery-fixtures/$TASK-$LABEL
OUT=$R/docs/evidence/$TASK/$LABEL
PG=$ROOT/restored-tools/pgdist/usr/lib/postgresql/18/bin
REDIS=$ROOT/restored-tools/redis-8.2.10/src
export LD_LIBRARY_PATH=$ROOT/restored-tools/pgdist/usr/lib/x86_64-linux-gnu
export PATH="$PG:$ROOT/restored-tools/go/bin:$ROOT/restored-tools/gopath/bin:$REDIS:$PATH"
export GOPATH=$ROOT/restored-tools/gopath GOCACHE=$ROOT/restored-tools/go-cache GOTOOLCHAIN=local
[[ ! -e "$FIX" && ! -e "$OUT" ]] || exit 2
mkdir -p "$FIX" "$OUT"
cp "$0" "$OUT/runner.sh"
git -C "$R" rev-parse HEAD > "$OUT/source-head.txt"
git -C "$R" diff --binary HEAD -- services db > "$OUT/source.patch"
"$PG/initdb" -D "$FIX/pgdata" -L "$ROOT/restored-tools/pgdist/usr/share/postgresql/18" -U weaveos_test -A trust --no-locale --encoding=UTF8 > "$FIX/initdb.log"
"$PG/pg_ctl" -D "$FIX/pgdata" -l "$FIX/pg.log" -o "-h 127.0.0.1 -p 55444 -k ''" start >/dev/null
cleanup(){ "$PG/pg_ctl" -D "$FIX/b2pgdata" stop -m fast >/dev/null || true; "$REDIS/redis-cli" -p 56444 shutdown nosave >/dev/null || true; "$PG/pg_ctl" -D "$FIX/pgdata" stop -m fast >/dev/null; }
trap cleanup EXIT
B2SOCKET=$(mktemp -d /tmp/weaveos-b2-v068.XXXXXX)
"$PG/initdb" -D "$FIX/b2pgdata" -L "$ROOT/restored-tools/pgdist/usr/share/postgresql/18" -U weaveos_test -A trust --no-locale --encoding=UTF8 > "$FIX/b2-initdb.log"
"$PG/pg_ctl" -D "$FIX/b2pgdata" -l "$FIX/b2-pg.log" -o "-h '' -p 55445 -k '$B2SOCKET'" start >/dev/null
createdb -h "$B2SOCKET" -p 55445 -U weaveos_test weaveos_b2_isolated_test
export WEAVEOS_B2_TEST_DATABASE_URL="postgres://weaveos_test@/weaveos_b2_isolated_test?host=$B2SOCKET&port=55445&sslmode=disable"
printf '%s\n' "$B2SOCKET" > "$OUT/b2-socket.txt"
"$REDIS/redis-server" --bind 127.0.0.1 --port 56444 --save '' --appendonly no --daemonize yes --pidfile "$FIX/redis.pid" --logfile "$FIX/redis.log" --dir "$FIX"
createdb -h 127.0.0.1 -p 55444 -U weaveos_test weaveos_ci_test
createdb -h 127.0.0.1 -p 55444 -U weaveos_test weaveos_ci_archive_test
export WEAVEOS_TEST_DATABASE_URL='postgres://weaveos_test@127.0.0.1:55444/weaveos_ci_test?sslmode=disable'
export WEAVEOS_TEST_ARCHIVE_DATABASE_URL='postgres://weaveos_test@127.0.0.1:55444/weaveos_ci_archive_test?sslmode=disable'
export WEAVEOS_TEST_REDIS_URL='redis://127.0.0.1:56444/15'
goose -dir "$R/db/archive-migrations" postgres "$WEAVEOS_TEST_ARCHIVE_DATABASE_URL" up > "$OUT/migrations.txt" 2>&1
goose -dir "$R/db/migrations" postgres "$WEAVEOS_TEST_DATABASE_URL" up >> "$OUT/migrations.txt" 2>&1
psql "$WEAVEOS_TEST_DATABASE_URL" -v ON_ERROR_STOP=1 -f "$R/infra/runtime/roles.sql" > "$OUT/roles.txt"
# Deliberately do not install cold roles globally: each fixture must own its setup.
{ postgres --version; redis-server --version; go version; } > "$OUT/environment.txt"
cd "$R/services/bff"
if [[ $MODE == red ]]; then
 cmd=(go test -race -p 1 -count=1 -json -run '^(TestRootDeletionCompletionArchivesWithOriginalBytes|TestControlledMaintenanceAndReaderUseTheirActualRestrictedRoles)$' ./internal/appstructure ./internal/audit)
else
 cmd=(go test -race -p 1 -count=1 -json ./...)
fi
printf '%q ' "${cmd[@]}" > "$OUT/command.txt"
date -u +%FT%TZ > "$OUT/started.txt"
set +e
"${cmd[@]}" > "$OUT/go.jsonl" 2>&1
code=$?
set -e
echo "$code" > "$OUT/exit.txt";date -u +%FT%TZ > "$OUT/finished.txt"
python - "$OUT/go.jsonl" <<'PY'
import json,sys,collections
c=collections.Counter()
for line in open(sys.argv[1]):
 try:r=json.loads(line)
 except ValueError:continue
 if r.get('Action') in ['pass','fail','skip'] and r.get('Test'):c[(r['Action'],'sub' if '/' in r['Test'] else 'top')]+=1
 if r.get('Action')=='fail':print(r)
print(c)
PY
exit "$code"
