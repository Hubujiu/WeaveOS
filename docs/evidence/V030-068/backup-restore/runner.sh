#!/bin/bash
set -euo pipefail
ROOT=/workspace/scratch/75c2273f6a80
R=$ROOT/WeaveOS-worktrees/V030-068
FIX=$ROOT/app-recovery-fixtures/V030-068-cold-archive-green
OUT=$R/docs/evidence/V030-068/backup-restore
PG=$ROOT/restored-tools/pgdist/usr/lib/postgresql/18/bin
export LD_LIBRARY_PATH=$ROOT/restored-tools/pgdist/usr/lib/x86_64-linux-gnu
export PATH="$PG:$ROOT/restored-tools/gopath/bin:$PATH"
[[ ! -e "$OUT" ]]||exit 2
mkdir "$OUT";cp "$0" "$OUT/runner.sh"
"$PG/pg_ctl" -D "$FIX/pgdata" -l "$FIX/backup.log" -o "-h 127.0.0.1 -p 55444 -k ''" start >/dev/null
trap '"$PG/pg_ctl" -D "$FIX/pgdata" stop -m fast >/dev/null' EXIT
SRC='postgres://weaveos_test@127.0.0.1:55444/weaveos_ci_test?sslmode=disable'
COLD='postgres://weaveos_test@127.0.0.1:55444/weaveos_ci_archive_test?sslmode=disable'
DST='postgres://weaveos_test@127.0.0.1:55444/weaveos_deletion_restored?sslmode=disable'
date -u +%FT%TZ > "$OUT/started.txt"
# Owner-created synthetic states test dump/restore, not execution correctness.
psql "$SRC" -v ON_ERROR_STOP=1 > "$OUT/fixture.txt" <<'SQL'
BEGIN;
CREATE TEMP TABLE backup_flows AS SELECT id FROM applications.workflow_definitions ORDER BY id LIMIT 3;
UPDATE applications.workflow_definitions SET state='closing',revision=revision+1,close_epoch=close_epoch+1 WHERE id IN(SELECT id FROM backup_flows);
INSERT INTO applications.workflow_deletions(flow_id,app_id,table_id,view_id,actor_user_id,operation_id,flow_name,expected_revision,fingerprint)
SELECT d.id,d.app_id,d.table_id,d.view_id,a.owner_user_id,gen_random_uuid(),d.name,d.revision-1,decode(repeat('ab',32),'hex') FROM applications.workflow_definitions d JOIN backup_flows f ON f.id=d.id JOIN applications.apps a ON a.id=d.app_id;
UPDATE applications.workflow_deletions SET status='unknown',reason='engine_unavailable' WHERE flow_id=(SELECT id FROM backup_flows ORDER BY id OFFSET 1 LIMIT 1);
UPDATE applications.workflow_deletions SET engine_deleted_versions=0,engine_deleted_at='2026-10-10T00:00:00.123456Z',status='deleted',completed_at='2026-10-10T00:00:01.123456Z' WHERE flow_id=(SELECT id FROM backup_flows ORDER BY id OFFSET 2 LIMIT 1);
COMMIT;
SQL
SQL="COPY (SELECT row_to_json(d)::text FROM applications.workflow_deletions d ORDER BY flow_id) TO STDOUT"
psql "$SRC" -v ON_ERROR_STOP=1 -c "$SQL" > "$FIX/source-deletions.jsonl"
[[ $(wc -l < "$FIX/source-deletions.jsonl") == 3 ]]
pg_dump "$SRC" --role=auth_backup --format=custom --file="$FIX/deletion.dump" 2> "$OUT/dump.stderr"
createdb -h 127.0.0.1 -p 55444 -U weaveos_test weaveos_deletion_restored
pg_restore --exit-on-error --dbname="$DST" "$FIX/deletion.dump" > "$OUT/restore.txt" 2>&1
psql "$DST" -v ON_ERROR_STOP=1 -c "$SQL" > "$FIX/restored-deletions.jsonl"
cmp "$FIX/source-deletions.jsonl" "$FIX/restored-deletions.jsonl"
sha256sum "$FIX/source-deletions.jsonl" "$FIX/restored-deletions.jsonl" > "$OUT/identity-sha256.txt"
psql "$DST" -v ON_ERROR_STOP=1 -Atc "SELECT status,count(*) FROM applications.workflow_deletions GROUP BY status ORDER BY status; SELECT has_table_privilege('auth_app','applications.workflow_deletions','DELETE'),has_column_privilege('auth_app','applications.workflow_deletions','flow_name','UPDATE'),has_function_privilege('auth_app','applications.complete_workflow_deletion(uuid,uuid,uuid,uuid)','EXECUTE'); SELECT tgname FROM pg_trigger WHERE tgname IN('guard_workflow_deletion_identity','guard_deleted_workflow_configuration','guard_deleted_workflow_version') ORDER BY tgname;" > "$OUT/restored-shape.txt"
# Both nonempty Downs must refuse and preserve their version.
set +e
goose -dir "$R/db/migrations" postgres "$SRC" down > "$OUT/nonempty-hot-down.txt" 2>&1;hot=$?
goose -dir "$R/db/archive-migrations" postgres "$COLD" down > "$OUT/nonempty-cold-down.txt" 2>&1;cold=$?
set -e
[[ "$hot" != 0 && "$cold" != 0 ]];grep -q 55000 "$OUT/nonempty-hot-down.txt";grep -q 55000 "$OUT/nonempty-cold-down.txt"
for name in hot cold;do
 createdb -h 127.0.0.1 -p 55444 -U weaveos_test "weaveos_deletion_empty_$name"
 url="postgres://weaveos_test@127.0.0.1:55444/weaveos_deletion_empty_$name?sslmode=disable"
 dir="$R/db/migrations";[[ "$name" == cold ]]&&dir="$R/db/archive-migrations"
 goose -dir "$dir" postgres "$url" up > "$OUT/empty-$name-cycle.txt" 2>&1
 goose -dir "$dir" postgres "$url" down >> "$OUT/empty-$name-cycle.txt" 2>&1
 goose -dir "$dir" postgres "$url" up >> "$OUT/empty-$name-cycle.txt" 2>&1
done
psql "$SRC" -Atc 'SELECT count(*) FROM applications.workflow_deletions' > "$OUT/post-down-count.txt"
date -u +%FT%TZ > "$OUT/finished.txt";echo 0 > "$OUT/exit.txt";cat "$OUT/restored-shape.txt"
