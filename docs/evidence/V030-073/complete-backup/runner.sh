#!/bin/bash
set -euo pipefail
ROOT=/workspace/scratch/75c2273f6a80
R=$ROOT/WeaveOS-worktrees/V030-073
FIX=$ROOT/app-recovery-fixtures/V030-073-backup-seed
OUT=$R/docs/evidence/V030-073/complete-backup
PG=$ROOT/restored-tools/pgdist/usr/lib/postgresql/18/bin
export LD_LIBRARY_PATH=$ROOT/restored-tools/pgdist/usr/lib/x86_64-linux-gnu
export PATH="$PG:$ROOT/restored-tools/gopath/bin:$PATH"
[[ ! -e "$OUT" && ! -e "$FIX/pgdata/postmaster.pid" ]] || exit 2
mkdir "$OUT";cp "$0" "$OUT/runner.sh";git -C "$R" rev-parse HEAD > "$OUT/source-head.txt"
git -C "$R" diff --binary HEAD -- services db > "$OUT/source.patch"
"$PG/pg_ctl" -D "$FIX/pgdata" -l "$FIX/backup.log" -o "-h 127.0.0.1 -p 55458 -k ''" start >/dev/null
trap '"$PG/pg_ctl" -D "$FIX/pgdata" stop -m fast >/dev/null' EXIT
SRC='postgres://weaveos_test@127.0.0.1:55458/weaveos_ci_test?sslmode=disable'
DST='postgres://weaveos_test@127.0.0.1:55458/weaveos_structure_complete_restored?sslmode=disable'
date -u +%FT%TZ > "$OUT/started.txt"
cat > "$OUT/snapshot.sql" <<'SQL'
SELECT format('SELECT %L,COALESCE(jsonb_agg(to_jsonb(r) ORDER BY to_jsonb(r)::text),%L) FROM %I.%I r;',schemaname||'.'||tablename,'[]',schemaname,tablename) FROM pg_tables WHERE schemaname IN ('auth','personnel','appdata','applications') ORDER BY schemaname,tablename
\gexec
SELECT 'roles',jsonb_build_array(has_table_privilege('auth_backup','applications.structure_deletions','SELECT'),has_table_privilege('auth_app','applications.structure_deletions','SELECT'),has_table_privilege('auth_app','applications.structure_deletions','UPDATE'),has_table_privilege('auth_reader','applications.structure_deletions','SELECT'),has_column_privilege('auth_app','applications.apps','deleted_at','UPDATE'),has_column_privilege('auth_app','applications.form_views','deleted_at','UPDATE'));
SQL
psql "$SRC" -v ON_ERROR_STOP=1 -Atf "$OUT/snapshot.sql" > "$FIX/source-structure-snapshot.txt"
psql "$SRC" -Atc "SELECT resource_kind,count(*) FROM applications.structure_deletions GROUP BY resource_kind ORDER BY resource_kind;" > "$OUT/kind-counts.txt"
for kind in application directory table form;do grep -Eq "^$kind\|[1-9][0-9]*$" "$OUT/kind-counts.txt";done
psql "$SRC" -Atc "SELECT count(*) FROM applications.structure_deletions;SELECT count(*) FROM applications.operations WHERE operation_kind IN ('application.delete','directory.delete','table.delete','form.delete');" > "$OUT/durable-counts.txt"
[[ $(head -1 "$OUT/durable-counts.txt") -gt 0 ]]
[[ $(head -1 "$OUT/durable-counts.txt") = $(tail -1 "$OUT/durable-counts.txt") ]]
pg_dump "$SRC" --role=auth_backup --format=custom --file="$FIX/structure.dump" 2> "$OUT/dump.stderr"
createdb -h 127.0.0.1 -p 55458 -U weaveos_test weaveos_structure_complete_restored
pg_restore --exit-on-error --dbname="$DST" "$FIX/structure.dump" > "$OUT/restore.txt" 2>&1
psql "$DST" -v ON_ERROR_STOP=1 -Atf "$OUT/snapshot.sql" > "$FIX/restored-structure-snapshot.txt"
cmp "$FIX/source-structure-snapshot.txt" "$FIX/restored-structure-snapshot.txt"
sha256sum "$FIX/source-structure-snapshot.txt" "$FIX/restored-structure-snapshot.txt" > "$OUT/snapshot-sha256.txt"
psql "$DST" -Atc "SELECT has_table_privilege('auth_backup','applications.structure_deletions','SELECT'),has_table_privilege('auth_app','applications.structure_deletions','SELECT'),has_table_privilege('auth_app','applications.structure_deletions','UPDATE'),has_table_privilege('auth_reader','applications.structure_deletions','SELECT');SELECT max(version_id) FROM goose_db_version WHERE is_applied;" > "$OUT/restored-shape.txt"
grep -q '^t|t|f|f$' "$OUT/restored-shape.txt";grep -q '^35$' "$OUT/restored-shape.txt"
printf '%s\n' 'Native restricted hot35 complete auth/personnel/applications/appdata snapshots, actual four-kind deletion receipts, retained typed records and history. Encrypted container packaging still requires final CI.' > "$OUT/scope.txt"
date -u +%FT%TZ > "$OUT/finished.txt";echo 0 > "$OUT/exit.txt";cat "$OUT/kind-counts.txt" "$OUT/restored-shape.txt"
