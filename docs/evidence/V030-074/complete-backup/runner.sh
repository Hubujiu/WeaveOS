#!/bin/bash
set -euo pipefail
ROOT=/workspace/scratch/75c2273f6a80
R=$ROOT/WeaveOS-worktrees/V030-074
FIX=$ROOT/v074-root-fixture
OUT=$R/docs/evidence/V030-074/complete-backup
source "$FIX/env.sh"
[[ ! -e "$OUT" ]] || exit 2
mkdir "$OUT";cp "$0" "$OUT/runner.sh";git -C "$R" rev-parse HEAD > "$OUT/source-head.txt";date -u +%FT%TZ > "$OUT/started.txt"
pg_ctl -D "$FIX/pgdata" -l "$FIX/backup.log" -o "-h 127.0.0.1 -p 55460 -c unix_socket_directories=''" start >/dev/null
trap 'pg_ctl -D "$FIX/pgdata" stop -m fast >/dev/null' EXIT
SRC="$WEAVEOS_TEST_DATABASE_URL"
DST='postgres://weaveos_test@127.0.0.1:55460/weaveos_copy_complete_restored?sslmode=disable'
cat > "$OUT/snapshot.sql" <<'SQL'
SELECT format('SELECT %L,COALESCE(jsonb_agg(to_jsonb(r) ORDER BY to_jsonb(r)::text),%L) FROM %I.%I r;',schemaname||'.'||tablename,'[]',schemaname,tablename) FROM pg_tables WHERE schemaname IN ('auth','personnel','appdata','applications') ORDER BY schemaname,tablename
\gexec
SELECT 'copy-role',has_table_privilege('auth_backup','applications.operations','SELECT'),has_table_privilege('auth_app','applications.operations','DELETE'),has_schema_privilege('auth_app','appdata','CREATE');
SQL
psql "$SRC" -v ON_ERROR_STOP=1 -Atf "$OUT/snapshot.sql" > "$FIX/source-copy-snapshot.txt"
psql "$SRC" -Atc "SELECT result_json->>'resourceKind',count(*) FROM applications.operations WHERE operation_kind='structure.copy' GROUP BY 1 ORDER BY 1;" > "$OUT/kind-counts.txt"
for kind in application directory table form;do grep -Eq "^$kind\|[1-9][0-9]*$" "$OUT/kind-counts.txt";done
pg_dump "$SRC" --role=auth_backup --format=custom --compress=none --file="$FIX/copy.dump" 2> "$OUT/dump.stderr"
createdb -h 127.0.0.1 -p 55460 -U weaveos_test weaveos_copy_complete_restored
pg_restore --exit-on-error --dbname="$DST" "$FIX/copy.dump" > "$OUT/restore.txt" 2>&1
psql "$DST" -v ON_ERROR_STOP=1 -Atf "$OUT/snapshot.sql" > "$FIX/restored-copy-snapshot.txt"
cmp "$FIX/source-copy-snapshot.txt" "$FIX/restored-copy-snapshot.txt"
sha256sum "$FIX/source-copy-snapshot.txt" "$FIX/restored-copy-snapshot.txt" > "$OUT/snapshot-sha256.txt"
psql "$DST" -Atc "SELECT max(version_id) FROM goose_db_version WHERE is_applied;SELECT count(*) FROM applications.operations WHERE operation_kind='structure.copy';SELECT count(*) FROM applications.workflow_definitions WHERE state='disabled' AND current_version=0 AND candidate_version=1;" > "$OUT/restored-shape.txt"
grep -q '^36$' "$OUT/restored-shape.txt"
printf '%s\n' 'Actual four-kind service copies: restricted auth_backup native hot36 complete auth/personnel/applications/appdata snapshot restored byte-for-byte, including typed tables, mapped groups, disabled candidate workflows, audit and receipts. Container encryption packaging requires final CI; no production data.' > "$OUT/scope.txt"
date -u +%FT%TZ > "$OUT/finished.txt";echo 0 > "$OUT/exit.txt";cat "$OUT/kind-counts.txt" "$OUT/snapshot-sha256.txt" "$OUT/restored-shape.txt"
