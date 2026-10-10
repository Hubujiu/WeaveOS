#!/bin/bash
set -euo pipefail
ROOT=/workspace/scratch/75c2273f6a80
TASK=V030-069; LABEL=$1; COUNT=${2:-5}
[[ "$COUNT" =~ ^[1-9][0-9]*$ ]] || exit 2
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
cleanup(){ "$REDIS/redis-cli" -p 56444 shutdown nosave >/dev/null || true; "$PG/pg_ctl" -D "$FIX/pgdata" stop -m fast >/dev/null; }
trap cleanup EXIT
"$REDIS/redis-server" --bind 127.0.0.1 --port 56444 --save '' --appendonly no --daemonize yes --pidfile "$FIX/redis.pid" --logfile "$FIX/redis.log" --dir "$FIX"
createdb -h 127.0.0.1 -p 55444 -U weaveos_test weaveos_ci_test
createdb -h 127.0.0.1 -p 55444 -U weaveos_test weaveos_ci_archive_test
export WEAVEOS_TEST_DATABASE_URL='postgres://weaveos_test@127.0.0.1:55444/weaveos_ci_test?sslmode=disable'
export WEAVEOS_TEST_ARCHIVE_DATABASE_URL='postgres://weaveos_test@127.0.0.1:55444/weaveos_ci_archive_test?sslmode=disable'
export WEAVEOS_TEST_REDIS_URL='redis://127.0.0.1:56444/15'
goose -dir "$R/db/archive-migrations" postgres "$WEAVEOS_TEST_ARCHIVE_DATABASE_URL" up > "$OUT/migrations.txt" 2>&1
goose -dir "$R/db/migrations" postgres "$WEAVEOS_TEST_DATABASE_URL" up >> "$OUT/migrations.txt" 2>&1
psql "$WEAVEOS_TEST_DATABASE_URL" -v ON_ERROR_STOP=1 -f "$R/infra/runtime/roles.sql" > "$OUT/roles.txt"
psql "$WEAVEOS_TEST_ARCHIVE_DATABASE_URL" -v ON_ERROR_STOP=1 -f "$R/infra/runtime/cold-roles.sql" >> "$OUT/roles.txt"
{ postgres --version; redis-server --version; go version; } > "$OUT/environment.txt"
cd "$R/services/bff"
psql "$WEAVEOS_TEST_DATABASE_URL" -v ON_ERROR_STOP=1 -c "CREATE ROLE b3_fixture SUPERUSER LOGIN PASSWORD 'b3_fixture_only';" -c 'CREATE DATABASE b3_flowable_fixture OWNER b3_fixture;' > "$OUT/engine-setup.txt"
psql 'postgres://b3_fixture:b3_fixture_only@127.0.0.1:55444/b3_flowable_fixture?sslmode=disable' -c 'CREATE SCHEMA workflow AUTHORIZATION b3_fixture;' >> "$OUT/engine-setup.txt"
goose -dir "$R/services/workflow-engine/schema/migrations" -table workflow.goose_db_version postgres 'postgres://b3_fixture:b3_fixture_only@127.0.0.1:55444/b3_flowable_fixture?sslmode=disable&search_path=workflow' up >> "$OUT/engine-setup.txt" 2>&1
psql 'postgres://b3_fixture:b3_fixture_only@127.0.0.1:55444/b3_flowable_fixture?sslmode=disable' -v ON_ERROR_STOP=1 -f "$R/services/workflow-engine/schema/formal-runtime-fixture-role.sql" >> "$OUT/engine-setup.txt"
# Local isolated PG must use original formal fixture's 5432 JDBC port.
"$PG/pg_ctl" -D "$FIX/pgdata" stop -m fast >/dev/null
"$PG/pg_ctl" -D "$FIX/pgdata" -l "$FIX/pg.log" -o "-h 127.0.0.1 -p 5432 -k ''" start >/dev/null
export WEAVEOS_TEST_DATABASE_URL=${WEAVEOS_TEST_DATABASE_URL/55444/5432}
export WEAVEOS_TEST_ARCHIVE_DATABASE_URL=${WEAVEOS_TEST_ARCHIVE_DATABASE_URL/55444/5432}
printf '127.0.0.1 localhost b3-postgres\n' > "$FIX/java-hosts"
mkdir "$FIX/bin"
JAVA=$(cat "$ROOT/restored-tools/jdk17-path.txt")/bin/java
printf '#!/bin/sh\nexec "%s" "-Djdk.net.hosts.file=%s" "$@"\n' "$JAVA" "$FIX/java-hosts" > "$FIX/bin/java"
chmod +x "$FIX/bin/java"
export PATH="$FIX/bin:$PATH"
export WEAVEOS_FORMAL_JAVA_CLASSPATH="$R/services/workflow-engine/target/classes:$(cat "$ROOT/restored-tools/v067-classpath")"
export WEAVEOS_FORMAL_BFF_BINARY="$FIX/formal-bff"
CGO_ENABLED=0 go build -o "$WEAVEOS_FORMAL_BFF_BINARY" ./cmd/bff
cmd=(go test -count="$COUNT" -json -tags workflowruntime_integration,workflowrpc_integration -run '^TestRootFormalRuntime' -timeout 600s ./cmd/bff)
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
