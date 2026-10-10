#!/bin/bash
set -euo pipefail
ROOT=/workspace/scratch/75c2273f6a80
R=$ROOT/WeaveOS-worktrees/V030-071
FIX=$ROOT/app-recovery-fixtures/V030-071-formal-presets-final
OUT=$R/docs/evidence/V030-071/formal-backup-restore
PG=$ROOT/restored-tools/pgdist/usr/lib/postgresql/18/bin
export LD_LIBRARY_PATH=$ROOT/restored-tools/pgdist/usr/lib/x86_64-linux-gnu
export PATH="$PG:$ROOT/restored-tools/gopath/bin:$PATH"
[[ ! -e "$OUT" && ! -e "$FIX/pgdata/postmaster.pid" ]] || exit 2
mkdir "$OUT";cp "$0" "$OUT/runner.sh";git -C "$R" rev-parse HEAD > "$OUT/source-head.txt"
git -C "$R" diff --binary HEAD -- services > "$OUT/source.patch"
"$PG/pg_ctl" -D "$FIX/pgdata" -l "$FIX/backup.log" -o "-h 127.0.0.1 -p 55446 -k ''" start >/dev/null
trap '"$PG/pg_ctl" -D "$FIX/pgdata" stop -m fast >/dev/null' EXIT
SRC='postgres://weaveos_test@127.0.0.1:55446/weaveos_ci_test?sslmode=disable'
DST='postgres://weaveos_test@127.0.0.1:55446/weaveos_presets_restored?sslmode=disable'
date -u +%FT%TZ > "$OUT/started.txt"
cat > "$OUT/snapshot.sql" <<'SQL'
SELECT 'private',COALESCE(jsonb_agg(to_jsonb(p) ORDER BY p.id),'[]') FROM applications.table_presets p;
SELECT 'receipts',COALESCE(jsonb_agg(to_jsonb(o) ORDER BY o.actor_user_id,o.operation_id),'[]') FROM applications.operations o WHERE operation_kind IN('preset.create','preset.update','preset.discard');
SELECT 'legacy',COALESCE(jsonb_agg(to_jsonb(p) ORDER BY p.id),'[]') FROM personnel.table_presets p;
SELECT 'fields',COALESCE(jsonb_agg(to_jsonb(f) ORDER BY f.id),'[]') FROM applications.fields f;
SELECT 'forms',COALESCE(jsonb_agg(to_jsonb(f) ORDER BY f.id),'[]') FROM applications.form_views f;
SELECT 'tables',COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.id),'[]') FROM applications.logical_tables t;
SELECT 'roles',jsonb_build_array(has_table_privilege('auth_backup','applications.table_presets','SELECT'),has_column_privilege('auth_app','applications.table_presets','name','UPDATE'),has_column_privilege('auth_app','applications.table_presets','owner_user_id','UPDATE'),has_table_privilege('auth_reader','applications.table_presets','SELECT'));
SQL
psql "$SRC" -v ON_ERROR_STOP=1 -Atf "$OUT/snapshot.sql" > "$FIX/source-preset-snapshot.txt"
psql "$SRC" -Atc "SELECT count(*) FROM applications.table_presets; SELECT count(*) FROM applications.operations WHERE operation_kind IN('preset.create','preset.update','preset.discard');" > "$OUT/durable-counts.txt"
[[ $(head -1 "$OUT/durable-counts.txt") -ge 20 ]]
pg_dump "$SRC" --role=auth_backup --format=custom --file="$FIX/private-presets.dump" 2> "$OUT/dump.stderr"
createdb -h 127.0.0.1 -p 55446 -U weaveos_test weaveos_presets_restored
pg_restore --exit-on-error --dbname="$DST" "$FIX/private-presets.dump" > "$OUT/restore.txt" 2>&1
psql "$DST" -v ON_ERROR_STOP=1 -Atf "$OUT/snapshot.sql" > "$FIX/restored-preset-snapshot.txt"
cmp "$FIX/source-preset-snapshot.txt" "$FIX/restored-preset-snapshot.txt"
sha256sum "$FIX/source-preset-snapshot.txt" "$FIX/restored-preset-snapshot.txt" > "$OUT/snapshot-sha256.txt"
psql "$DST" -Atc "SELECT has_table_privilege('auth_backup','applications.table_presets','SELECT'),has_column_privilege('auth_app','applications.table_presets','name','UPDATE'),has_column_privilege('auth_app','applications.table_presets','owner_user_id','UPDATE'),has_table_privilege('auth_reader','applications.table_presets','SELECT'); SELECT max(version_id) FROM goose_db_version WHERE is_applied;" > "$OUT/restored-shape.txt"
grep -q '^t|t|f|f$' "$OUT/restored-shape.txt";grep -q '^33$' "$OUT/restored-shape.txt"
printf '%s\n' 'Formal hot33 schema and reviewed role backup/restore; packaging remains final CI verification.' > "$OUT/scope.txt"
date -u +%FT%TZ > "$OUT/finished.txt";echo 0 > "$OUT/exit.txt";cat "$OUT/durable-counts.txt" "$OUT/restored-shape.txt"
