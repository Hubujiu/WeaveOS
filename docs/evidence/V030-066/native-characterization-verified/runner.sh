#!/bin/bash
set -euo pipefail
ROOT=/workspace/scratch/75c2273f6a80
LABEL=$1
[[ "$LABEL" =~ ^[a-z0-9-]+$ ]] || exit 2
REPO=$ROOT/WeaveOS-worktrees/V030-066
FIX=$ROOT/v066-fixture/$LABEL
OUT=$REPO/docs/evidence/V030-066/$LABEL
PG=$ROOT/restored-tools/pg/bin
[[ ! -e "$FIX" ]] || exit 2
mkdir -p "$FIX" "$OUT"
"$PG/initdb" -D "$FIX/pgdata" -U weaveos_test -A trust --no-locale --encoding=UTF8 > "$FIX/initdb.log"
"$PG/pg_ctl" -D "$FIX/pgdata" -l "$FIX/postgres.log" -o "-h 127.0.0.1 -p 55445 -k ''" start >/dev/null
trap '"$PG/pg_ctl" -D "$FIX/pgdata" stop -m fast >/dev/null' EXIT
"$PG/psql" 'postgres://weaveos_test@127.0.0.1:55445/postgres?sslmode=disable' -v ON_ERROR_STOP=1 -c "CREATE ROLE b3_fixture LOGIN PASSWORD 'b3_fixture_only';" -c 'CREATE DATABASE weaveos_v066_engine_test OWNER b3_fixture;' > "$FIX/db.log"
export WEAVEOS_V066_TEST_JDBC_URL='jdbc:postgresql://127.0.0.1:55445/weaveos_v066_engine_test'
export JAVA_HOME=$(cat "$ROOT/restored-tools/jdk17-path.txt")
export PATH="$JAVA_HOME/bin:$PATH"
cd "$REPO"
git rev-parse HEAD > "$OUT/source-head.txt"
git diff --binary HEAD -- services/workflow-engine > "$OUT/uncommitted-source.patch"
date -u +%FT%TZ > "$OUT/started.txt"
cmd=(python "$ROOT/run-maven-v066.py" "-Dworkflow.reports=$OUT/reports" -Dtest=RootNativeDeletionContractTest test)
printf '%q ' "${cmd[@]}" > "$OUT/command.txt"
set +e
"${cmd[@]}" > "$OUT/maven.txt" 2>&1
code=$?;set -e
echo "$code" > "$OUT/exit.txt";date -u +%FT%TZ > "$OUT/finished.txt"
tail -45 "$OUT/maven.txt"
exit "$code"
