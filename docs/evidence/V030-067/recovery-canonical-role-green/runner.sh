#!/bin/bash
set -euo pipefail
ROOT=/workspace/scratch/75c2273f6a80
LABEL=$1
[[ "$LABEL" =~ ^[a-z0-9-]+$ ]] || exit 2
REPO=$ROOT/WeaveOS-worktrees/V030-067
FIX=$ROOT/v067-recovery-fixtures/$LABEL
OUT=$REPO/docs/evidence/V030-067/$LABEL
PG=$ROOT/restored-tools/pgdist/usr/lib/postgresql/18/bin
export LD_LIBRARY_PATH=$ROOT/restored-tools/pgdist/usr/lib/x86_64-linux-gnu
[[ ! -e "$FIX" && ! -e "$OUT" ]] || exit 2
mkdir -p "$FIX" "$OUT"
cp "$0" "$OUT/runner.sh"
git -C "$REPO" rev-parse HEAD > "$OUT/source-head.txt"
git -C "$REPO" diff --binary HEAD -- services/workflow-engine > "$OUT/source.patch"
date -u +%FT%TZ > "$OUT/started.txt"
"$PG/initdb" -D "$FIX/data" -L "$ROOT/restored-tools/pgdist/usr/share/postgresql/18" -U fixture -A trust --no-locale --encoding=UTF8 > "$FIX/initdb.log"
"$PG/pg_ctl" -D "$FIX/data" -l "$FIX/pg.log" -o "-h 127.0.0.1 -p 55446 -k ''" start >/dev/null
trap '"$PG/pg_ctl" -D "$FIX/data" stop -m fast >/dev/null' EXIT
URL='postgres://fixture@127.0.0.1:55446/b3_flowable_fixture?sslmode=disable'
"$PG/createdb" -h 127.0.0.1 -p 55446 -U fixture b3_flowable_fixture
"$PG/psql" "$URL" -v ON_ERROR_STOP=1 -c 'CREATE SCHEMA workflow' > "$OUT/setup.txt"
PGOPTIONS='-c search_path=workflow' "$ROOT/restored-tools/gopath/bin/goose" -dir "$REPO/services/workflow-engine/schema/migrations" postgres "$URL" up > "$OUT/goose.txt" 2>&1
"$PG/psql" "$URL" -v ON_ERROR_STOP=1 -f "$REPO/services/workflow-engine/schema/formal-runtime-fixture-role.sql" >> "$OUT/setup.txt"
set +e
"$PG/psql" "$URL" -v ON_ERROR_STOP=1 > "$OUT/assertions.txt" 2>&1 <<'SQL'
SET ROLE v041_runtime;
SET search_path=workflow;
INSERT INTO wf_flow_deletion_guards(app_id,flow_id) VALUES('10000000-0000-4000-8000-000000000001','20000000-0000-4000-8000-000000000001');
UPDATE wf_flow_deletion_guards SET retired=true,operation_id='70000000-0000-4000-8000-000000000001',deleted_versions=0,deleted_at=clock_timestamp();
DO $$ BEGIN
IF NOT EXISTS(SELECT 1 FROM wf_flow_deletion_guards WHERE retired AND deleted_versions=0) THEN RAISE EXCEPTION 'missing original deletion'; END IF;
IF has_table_privilege(current_user,'wf_flow_deletion_guards','DELETE,TRUNCATE,UPDATE') OR has_column_privilege(current_user,'wf_flow_deletion_guards','app_id','UPDATE') OR has_column_privilege(current_user,'wf_flow_deletion_guards','created_at','UPDATE') THEN RAISE EXCEPTION 'excessive authority'; END IF;
END $$;
SQL
code=$?
set -e
echo "$code" > "$OUT/exit.txt"
date -u +%FT%TZ > "$OUT/finished.txt"
cat "$OUT/goose.txt" "$OUT/assertions.txt"
exit "$code"
