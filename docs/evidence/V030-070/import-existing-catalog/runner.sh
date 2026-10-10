#!/bin/bash
set -euo pipefail
ROOT=/workspace/scratch/75c2273f6a80
R=$ROOT/WeaveOS-worktrees/V030-070
FIX=$ROOT/app-recovery-fixtures/V030-070-import-full-regression
OUT=$R/docs/evidence/V030-070/import-existing-catalog
PG=$ROOT/restored-tools/pgdist/usr/lib/postgresql/18/bin
REDIS=$ROOT/restored-tools/redis-8.2.10/src
export LD_LIBRARY_PATH=$ROOT/restored-tools/pgdist/usr/lib/x86_64-linux-gnu
export PATH="$PG:$ROOT/restored-tools/go/bin:$ROOT/restored-tools/gopath/bin:$REDIS:$PATH"
export GOPATH=$ROOT/restored-tools/gopath GOCACHE=$ROOT/restored-tools/go-cache GOTOOLCHAIN=local
[[ ! -e "$OUT" ]] || exit 2
mkdir "$OUT";cp "$0" "$OUT/runner.sh";git -C "$R" rev-parse HEAD > "$OUT/source-head.txt";git -C "$R" diff HEAD -- services > "$OUT/source.patch"
"$PG/pg_ctl" -D "$FIX/pgdata" -l "$FIX/catalog-recheck.log" -o "-h 127.0.0.1 -p 55447 -k ''" start >/dev/null
cleanup(){ "$REDIS/redis-cli" -p 56447 shutdown nosave >/dev/null || true; "$PG/pg_ctl" -D "$FIX/pgdata" stop -m fast >/dev/null; };trap cleanup EXIT
"$REDIS/redis-server" --bind 127.0.0.1 --port 56447 --save '' --appendonly no --daemonize yes --pidfile "$FIX/catalog-redis.pid" --logfile "$FIX/catalog-redis.log" --dir "$FIX"
export WEAVEOS_TEST_DATABASE_URL='postgres://weaveos_test@127.0.0.1:55447/weaveos_ci_test?sslmode=disable'
export WEAVEOS_TEST_REDIS_URL='redis://127.0.0.1:56447/15'
psql "$WEAVEOS_TEST_DATABASE_URL" -Atc "SELECT count(*) FROM personnel.permission_catalog WHERE app_id NOT IN(SELECT id::text FROM applications.apps) AND category='application'" > "$OUT/preexisting-external-catalog-count.txt"
date -u +%FT%TZ > "$OUT/started.txt"
cd "$R/services/bff"
set +e
go test -race -p 1 -count=1 -json -run '^TestRootTemplateImportFaultRollsBack' ./cmd/bff > "$OUT/go.jsonl" 2>&1
code=$?
set -e
echo "$code" > "$OUT/exit.txt";date -u +%FT%TZ > "$OUT/finished.txt";cat "$OUT/preexisting-external-catalog-count.txt";tail -2 "$OUT/go.jsonl";exit "$code"
