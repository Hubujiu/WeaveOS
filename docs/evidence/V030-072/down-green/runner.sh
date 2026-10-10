#!/bin/bash
set -euo pipefail
ROOT=/workspace/scratch/75c2273f6a80
TASK=V030-072; LABEL=$1
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
"$PG/pg_ctl" -D "$FIX/pgdata" -l "$FIX/pg.log" -o "-h 127.0.0.1 -p 55450 -k ''" start >/dev/null
cleanup(){ "$REDIS/redis-cli" -p 56450 shutdown nosave >/dev/null || true; "$PG/pg_ctl" -D "$FIX/pgdata" stop -m fast >/dev/null; }
trap cleanup EXIT
"$REDIS/redis-server" --bind 127.0.0.1 --port 56450 --save '' --appendonly no --daemonize yes --pidfile "$FIX/redis.pid" --logfile "$FIX/redis.log" --dir "$FIX"
createdb -h 127.0.0.1 -p 55450 -U weaveos_test weaveos_ci_test
createdb -h 127.0.0.1 -p 55450 -U weaveos_test weaveos_ci_archive_test
export WEAVEOS_TEST_DATABASE_URL='postgres://weaveos_test@127.0.0.1:55450/weaveos_ci_test?sslmode=disable'
export WEAVEOS_TEST_ARCHIVE_DATABASE_URL='postgres://weaveos_test@127.0.0.1:55450/weaveos_ci_archive_test?sslmode=disable'
export WEAVEOS_TEST_REDIS_URL='redis://127.0.0.1:56450/15'
goose -dir "$R/db/archive-migrations" postgres "$WEAVEOS_TEST_ARCHIVE_DATABASE_URL" up > "$OUT/migrations.txt" 2>&1
goose -dir "$R/db/migrations" postgres "$WEAVEOS_TEST_DATABASE_URL" up >> "$OUT/migrations.txt" 2>&1
psql "$WEAVEOS_TEST_DATABASE_URL" -v ON_ERROR_STOP=1 -f "$R/infra/runtime/roles.sql" > "$OUT/roles.txt"
psql "$WEAVEOS_TEST_ARCHIVE_DATABASE_URL" -v ON_ERROR_STOP=1 -f "$R/infra/runtime/cold-roles.sql" >> "$OUT/roles.txt"
{ postgres --version; redis-server --version; go version; } > "$OUT/environment.txt"
cp "$R/db/migrations/00034_record_lifecycle.sql" "$OUT/migration.sql"
cp "$R/scripts/v030-072/check-migration.py" "$OUT/check-migration.py"
date -u +%FT%TZ > "$OUT/started.txt"
set +e
python "$R/scripts/v030-072/check-migration.py" "$OUT" > "$OUT/result.txt" 2>&1
code=$?
set -e
echo "$code" > "$OUT/exit.txt"
date -u +%FT%TZ > "$OUT/finished.txt"
cat "$OUT/result.txt"
exit "$code"
