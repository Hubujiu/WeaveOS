#!/bin/bash
set -euo pipefail
ROOT=/workspace/scratch/75c2273f6a80
PG=$ROOT/restored-tools/pg/bin
FIX=$ROOT/v065-fixture/history-advisory-green
OUT=$ROOT/WeaveOS-worktrees/V030-065/docs/evidence/V030-065/backup-restore
[[ ! -e "$OUT" ]] || { echo 'refuse overwrite'; exit 2; }
mkdir -p "$OUT"
"$PG/pg_ctl" -D "$FIX/pgdata" -l "$FIX/backup-postgres.log" -o "-h 127.0.0.1 -p 55444 -k ''" start >/dev/null
trap '"$PG/pg_ctl" -D "$FIX/pgdata" stop -m fast >/dev/null' EXIT
SRC='postgres://weaveos_test@127.0.0.1:55444/weaveos_v065?sslmode=disable'
DST='postgres://weaveos_test@127.0.0.1:55444/weaveos_v065_publication_restore?sslmode=disable'
date -u +%FT%TZ > "$OUT/started.txt"
SQL="COPY (SELECT t.name,row_to_json(h)::text FROM applications.workflow_publications h CROSS JOIN (VALUES ('workflow_publications')) t(name) UNION ALL SELECT 'workflow_engine_receipts',row_to_json(h)::text FROM applications.workflow_engine_receipts h UNION ALL SELECT 'workflow_deployments',row_to_json(h)::text FROM applications.workflow_deployments h ORDER BY 1,2) TO STDOUT"
"$PG/psql" "$SRC" -v ON_ERROR_STOP=1 -c "$SQL" > "$FIX/source-history.jsonl"
"$PG/pg_dump" "$SRC" --role=auth_backup --format=custom --file="$FIX/history-backup.dump" 2> "$OUT/dump.stderr"
"$PG/createdb" -h 127.0.0.1 -p 55444 -U weaveos_test weaveos_v065_publication_restore
"$PG/pg_restore" --exit-on-error --dbname="$DST" "$FIX/history-backup.dump" > "$OUT/restore.txt" 2>&1
"$PG/psql" "$DST" -v ON_ERROR_STOP=1 -c "$SQL" > "$FIX/restored-history.jsonl"
cmp "$FIX/source-history.jsonl" "$FIX/restored-history.jsonl"
sha256sum "$FIX/source-history.jsonl" "$FIX/restored-history.jsonl" > "$OUT/history-sha256.txt"
"$PG/psql" "$DST" -v ON_ERROR_STOP=1 -Atc "SELECT 'publications',count(*) FROM applications.workflow_publications UNION ALL SELECT 'full_receipts',count(*) FROM applications.workflow_engine_receipts UNION ALL SELECT 'legacy_deployments',count(*) FROM applications.workflow_deployments; SELECT tgname FROM pg_trigger WHERE tgname IN ('bind_publication_history_insert','bind_engine_receipt_history_insert','bind_deployment_history_insert','guard_workflow_definition_cleanup','guard_workflow_version_cleanup') ORDER BY tgname; SELECT has_table_privilege('auth_app','applications.workflow_versions','UPDATE'),has_table_privilege('auth_app','applications.workflow_engine_receipts','DELETE'),has_table_privilege('auth_backup','applications.workflow_publications','SELECT');" > "$OUT/restored-shape.txt"
date -u +%FT%TZ > "$OUT/finished.txt"
echo 0 > "$OUT/exit.txt"
cat "$OUT/restored-shape.txt"
