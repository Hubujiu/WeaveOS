#!/bin/bash
set -euo pipefail
ROOT=/workspace/scratch/75c2273f6a80
LABEL=$1
TESTS=${2:-RootFlowDeletionRegistryTest}
[[ "$LABEL" =~ ^[a-z0-9-]+$ ]] || exit 2
REPO=$ROOT/WeaveOS-worktrees/V030-067
FIX=$ROOT/v067-recovery-fixtures/$LABEL
OUT=$REPO/docs/evidence/V030-067/$LABEL
PG=$ROOT/restored-tools/pgdist/usr/lib/postgresql/18/bin
SHARE=$ROOT/restored-tools/pgdist/usr/share/postgresql/18
export LD_LIBRARY_PATH="$ROOT/restored-tools/pgdist/usr/lib/x86_64-linux-gnu${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
[[ ! -e "$FIX" && ! -e "$OUT" ]] || exit 2
mkdir -p "$FIX" "$OUT"
"$PG/initdb" -D "$FIX/pgdata" -L "$SHARE" -U weaveos_test -A trust --no-locale --encoding=UTF8 > "$FIX/initdb.log"
"$PG/pg_ctl" -D "$FIX/pgdata" -l "$FIX/postgres.log" -o "-h 127.0.0.1 -p 5432 -k ''" start >/dev/null
trap '"$PG/pg_ctl" -D "$FIX/pgdata" stop -m fast >/dev/null' EXIT
"$PG/psql" 'postgres://weaveos_test@127.0.0.1:5432/postgres?sslmode=disable' -v ON_ERROR_STOP=1 -c "CREATE ROLE b3_fixture SUPERUSER LOGIN PASSWORD 'b3_fixture_only';" -c 'CREATE DATABASE b3_flowable_fixture OWNER b3_fixture;' > "$FIX/db.log"
# Authenticate every synthetic runtime login; only the fixture administrator uses trust.
printf 'host all weaveos_test 127.0.0.1/32 trust\nhost all all 127.0.0.1/32 scram-sha-256\n' > "$FIX/host-auth"
cat "$FIX/pgdata/pg_hba.conf" >> "$FIX/host-auth"
cp "$FIX/host-auth" "$FIX/pgdata/pg_hba.conf"
"$PG/pg_ctl" -D "$FIX/pgdata" reload >/dev/null
export B3_TEST_JDBC_URL='jdbc:postgresql://b3-postgres:5432/b3_flowable_fixture'
printf '127.0.0.1 localhost b3-postgres\n' > "$FIX/java-test-hosts"
export JAVA_HOME=$(cat "$ROOT/restored-tools/jdk17-path.txt")
export PATH="$JAVA_HOME/bin:$PATH"
cd "$REPO"
git rev-parse HEAD > "$OUT/source-head.txt"
git diff --binary HEAD -- services/workflow-engine > "$OUT/uncommitted-source.patch"
cp "$ROOT/run-v067-legacy-tests.sh" "$OUT/runner.sh"
{ "$PG/postgres" --version; java -version; } > "$OUT/environment.txt" 2>&1
date -u +%FT%TZ > "$OUT/started.txt"
cmd=(python "$ROOT/run-maven-v067.py" "-DargLine=-Djdk.net.hosts.file=$FIX/java-test-hosts" "-Dworkflow.reports=$OUT/reports" "-Dtest=$TESTS" test)
printf '%q ' "${cmd[@]}" > "$OUT/command.txt"
set +e
"${cmd[@]}" > "$OUT/maven.txt" 2>&1
code=$?;set -e
echo "$code" > "$OUT/exit.txt";date -u +%FT%TZ > "$OUT/finished.txt"
tail -28 "$OUT/maven.txt"
exit "$code"
