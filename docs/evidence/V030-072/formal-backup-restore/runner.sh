#!/bin/bash
set -euo pipefail
ROOT=/workspace/scratch/75c2273f6a80
R=$ROOT/WeaveOS-worktrees/V030-072
FIX=$ROOT/app-recovery-fixtures/V030-072-atomic-corrected
OUT=$R/docs/evidence/V030-072/formal-backup-restore
PG=$ROOT/restored-tools/pgdist/usr/lib/postgresql/18/bin
export LD_LIBRARY_PATH=$ROOT/restored-tools/pgdist/usr/lib/x86_64-linux-gnu
export PATH="$PG:$ROOT/restored-tools/gopath/bin:$PATH"
[[ ! -e "$OUT" && ! -e "$FIX/pgdata/postmaster.pid" ]] || exit 2
mkdir "$OUT";cp "$0" "$OUT/runner.sh";git -C "$R" rev-parse HEAD > "$OUT/source-head.txt"
git -C "$R" diff --binary HEAD -- services > "$OUT/source.patch"
"$PG/pg_ctl" -D "$FIX/pgdata" -l "$FIX/backup.log" -o "-h 127.0.0.1 -p 55452 -k ''" start >/dev/null
trap '"$PG/pg_ctl" -D "$FIX/pgdata" stop -m fast >/dev/null' EXIT
SRC='postgres://weaveos_test@127.0.0.1:55452/weaveos_ci_test?sslmode=disable'
DST='postgres://weaveos_test@127.0.0.1:55452/weaveos_lifecycle_restored?sslmode=disable'
date -u +%FT%TZ > "$OUT/started.txt"
cat > "$OUT/snapshot.sql" <<'SQL'
SELECT 'lifecycle',COALESCE(jsonb_agg(to_jsonb(p) ORDER BY p.app_id,p.table_id,p.record_id),'[]') FROM applications.record_lifecycle p;
SELECT 'events',COALESCE(jsonb_agg(to_jsonb(p) ORDER BY p.id),'[]') FROM applications.record_lifecycle_events p;
SELECT 'operations',COALESCE(jsonb_agg(to_jsonb(p) ORDER BY p.actor_user_id,p.operation_id),'[]') FROM applications.operations p;
SELECT format('SELECT %L,COALESCE(jsonb_agg(to_jsonb(r) ORDER BY r.id),%L) FROM %I.%I r;',tablename,'[]',schemaname,tablename) FROM pg_tables WHERE schemaname='appdata' ORDER BY tablename
\gexec
SELECT 'roles',jsonb_build_array(has_table_privilege('auth_backup','applications.record_lifecycle','SELECT'),has_table_privilege('auth_app','applications.record_lifecycle','SELECT'),has_table_privilege('auth_app','applications.record_lifecycle','UPDATE'),has_table_privilege('auth_reader','applications.record_lifecycle','SELECT'));

SQL
psql "$SRC" -v ON_ERROR_STOP=1 -Atf "$OUT/snapshot.sql" > "$FIX/source-lifecycle-snapshot.txt"
psql "$SRC" -Atc "SELECT count(*) FROM applications.record_lifecycle; SELECT count(*) FROM applications.record_lifecycle_events; SELECT count(*) FROM applications.operations WHERE operation_kind IN('record.delete','record.restore');" > "$OUT/durable-counts.txt"
[[ $(head -1 "$OUT/durable-counts.txt") -ge 10 ]]
pg_dump "$SRC" --role=auth_backup --format=custom --file="$FIX/lifecycle.dump" 2> "$OUT/dump.stderr"
createdb -h 127.0.0.1 -p 55452 -U weaveos_test weaveos_lifecycle_restored
pg_restore --exit-on-error --dbname="$DST" "$FIX/lifecycle.dump" > "$OUT/restore.txt" 2>&1
psql "$DST" -v ON_ERROR_STOP=1 -Atf "$OUT/snapshot.sql" > "$FIX/restored-lifecycle-snapshot.txt"
cmp "$FIX/source-lifecycle-snapshot.txt" "$FIX/restored-lifecycle-snapshot.txt"
sha256sum "$FIX/source-lifecycle-snapshot.txt" "$FIX/restored-lifecycle-snapshot.txt" > "$OUT/snapshot-sha256.txt"
psql "$DST" -Atc "SELECT has_table_privilege('auth_backup','applications.record_lifecycle','SELECT'),has_table_privilege('auth_app','applications.record_lifecycle','SELECT'),has_table_privilege('auth_app','applications.record_lifecycle','UPDATE'),has_table_privilege('auth_reader','applications.record_lifecycle','SELECT'); SELECT max(version_id) FROM goose_db_version WHERE is_applied;" > "$OUT/restored-shape.txt"
grep -q '^t|t|f|f$' "$OUT/restored-shape.txt";grep -q '^34$' "$OUT/restored-shape.txt"
printf '%s\n' 'Formal hot34 typed rows, lifecycle states, events and operations; schema and reviewed role backup/restore; packaging remains final CI verification.' > "$OUT/scope.txt"
date -u +%FT%TZ > "$OUT/finished.txt";echo 0 > "$OUT/exit.txt";cat "$OUT/durable-counts.txt" "$OUT/restored-shape.txt"
