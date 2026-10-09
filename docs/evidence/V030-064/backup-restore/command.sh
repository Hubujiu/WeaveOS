#!/bin/bash
set -euo pipefail
ROOT=/workspace/scratch/75c2273f6a80
PG=$ROOT/restored-tools/pg/bin
FIX=$ROOT/v064-fixture/index-migration-green
OUT=$ROOT/WeaveOS-worktrees/V030-064/docs/evidence/V030-064/backup-restore
[[ ! -e "$OUT" ]] || { echo 'refuse overwrite'; exit 2; }
mkdir -p "$OUT"
"$PG/pg_ctl" -D "$FIX/pgdata" -l "$FIX/backup-postgres.log" -o "-h 127.0.0.1 -p 55444 -k ''" start >/dev/null
trap '"$PG/pg_ctl" -D "$FIX/pgdata" stop -m fast >/dev/null' EXIT
SRC='postgres://weaveos_test@127.0.0.1:55444/weaveos_v064?sslmode=disable'
DST='postgres://weaveos_test@127.0.0.1:55444/weaveos_v064_journal_restore?sslmode=disable'
date -u +%FT%TZ > "$OUT/started.txt"
SQL="COPY (SELECT row_to_json(e)::text FROM applications.workflow_execution_events e ORDER BY command_id) TO STDOUT"
"$PG/psql" "$SRC" -v ON_ERROR_STOP=1 -c "$SQL" > "$FIX/source-journal.jsonl"
"$PG/pg_dump" "$SRC" --role=auth_backup --format=custom --file="$FIX/journal-backup.dump" 2> "$OUT/dump.stderr"
"$PG/createdb" -h 127.0.0.1 -p 55444 -U weaveos_test weaveos_v064_journal_restore
"$PG/pg_restore" --exit-on-error --dbname="$DST" "$FIX/journal-backup.dump" > "$OUT/restore.txt" 2>&1
"$PG/psql" "$DST" -v ON_ERROR_STOP=1 -c "$SQL" > "$FIX/restored-journal.jsonl"
cmp "$FIX/source-journal.jsonl" "$FIX/restored-journal.jsonl"
sha256sum "$FIX/source-journal.jsonl" "$FIX/restored-journal.jsonl" > "$OUT/journal-sha256.txt"
"$PG/psql" "$DST" -v ON_ERROR_STOP=1 -Atc "SELECT count(*),count(*) FILTER(WHERE flow_name_source='captured'),count(*) FILTER(WHERE flow_name_source='legacy_last_known') FROM applications.workflow_execution_events; SELECT tgname FROM pg_trigger WHERE tgrelid='applications.workflow_execution_events'::regclass AND NOT tgisinternal ORDER BY tgname; SELECT indexname FROM pg_indexes WHERE schemaname='applications' AND tablename='workflow_execution_events' ORDER BY indexname; SELECT has_table_privilege('auth_app','applications.workflow_execution_events','SELECT'),has_table_privilege('auth_app','applications.workflow_execution_events','INSERT'),has_table_privilege('auth_app','applications.workflow_execution_events','UPDATE'),has_table_privilege('auth_app','applications.workflow_execution_events','DELETE'),has_table_privilege('auth_backup','applications.workflow_execution_events','SELECT');" > "$OUT/restored-shape.txt"
date -u +%FT%TZ > "$OUT/finished.txt"
echo 0 > "$OUT/exit.txt"
cat "$OUT/restored-shape.txt"
