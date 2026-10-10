#!/bin/bash
set -euo pipefail
ROOT=/workspace/scratch/75c2273f6a80
R=$ROOT/WeaveOS-worktrees/V030-070
FIX=$ROOT/app-recovery-fixtures/V030-070-import-roundtrip
OUT=$R/docs/evidence/V030-070/import-backup-restore
PG=$ROOT/restored-tools/pgdist/usr/lib/postgresql/18/bin
export LD_LIBRARY_PATH=$ROOT/restored-tools/pgdist/usr/lib/x86_64-linux-gnu
export PATH="$PG:$ROOT/restored-tools/gopath/bin:$PATH"
[[ ! -e "$OUT" ]] || exit 2
mkdir "$OUT";cp "$0" "$OUT/runner.sh";git -C "$R" rev-parse HEAD > "$OUT/source-head.txt"
"$PG/pg_ctl" -D "$FIX/pgdata" -l "$FIX/backup.log" -o "-h 127.0.0.1 -p 55446 -k ''" start >/dev/null
trap '"$PG/pg_ctl" -D "$FIX/pgdata" stop -m fast >/dev/null' EXIT
SRC='postgres://weaveos_test@127.0.0.1:55446/weaveos_ci_test?sslmode=disable'
DST='postgres://weaveos_test@127.0.0.1:55446/weaveos_template_restored?sslmode=disable'
date -u +%FT%TZ > "$OUT/started.txt"
cat > "$OUT/snapshot.sql" <<'SQL'
CREATE TEMP TABLE imported AS SELECT app_id FROM applications.operations WHERE operation_kind='application.template.import';
SELECT 'apps',COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.id),'[]') FROM applications.apps t WHERE t.id IN(SELECT app_id FROM imported);
SELECT 'receipts',COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.operation_id),'[]') FROM applications.operations t WHERE t.operation_kind='application.template.import';
SELECT 'directories',COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.id),'[]') FROM applications.directories t WHERE t.app_id IN(SELECT app_id FROM imported);
SELECT 'tables',COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.id),'[]') FROM applications.logical_tables t WHERE t.app_id IN(SELECT app_id FROM imported);
SELECT 'fields',COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.id),'[]') FROM applications.fields t WHERE t.app_id IN(SELECT app_id FROM imported);
SELECT 'forms',COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.id),'[]') FROM applications.form_views t WHERE t.app_id IN(SELECT app_id FROM imported);
SELECT 'groups',COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.id),'[]') FROM applications.permission_groups t WHERE t.app_id IN(SELECT app_id FROM imported);
SELECT 'members',COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.app_id,t.group_id,t.user_id),'[]') FROM applications.group_members t WHERE t.app_id IN(SELECT app_id FROM imported);
SELECT 'grants',COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.id),'[]') FROM applications.grants t WHERE t.app_id IN(SELECT app_id FROM imported);
SELECT 'grant_fields',COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.app_id,t.grant_id,t.field_id),'[]') FROM applications.grant_fields t WHERE t.app_id IN(SELECT app_id FROM imported);
SELECT 'flows',COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.id),'[]') FROM applications.workflow_definitions t WHERE t.app_id IN(SELECT app_id FROM imported);
SELECT 'versions',COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.flow_id,t.version),'[]') FROM applications.workflow_versions t WHERE t.app_id IN(SELECT app_id FROM imported);
SELECT 'physical',COALESCE(jsonb_agg(jsonb_build_array(t.id,a.attname,format_type(a.atttypid,a.atttypmod),a.attnotnull,pg_get_expr(d.adbin,d.adrelid)) ORDER BY t.id,a.attnum),'[]') FROM applications.logical_tables t JOIN pg_attribute a ON a.attrelid=to_regclass('appdata.t_'||replace(t.id::text,'-','')) AND a.attnum>0 AND NOT a.attisdropped LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum WHERE t.app_id IN(SELECT app_id FROM imported);
SQL
psql "$SRC" -v ON_ERROR_STOP=1 -Atf "$OUT/snapshot.sql" > "$FIX/source-template-snapshot.txt"
psql "$SRC" -Atc "SELECT count(*) FROM applications.operations WHERE operation_kind='application.template.import'" > "$OUT/import-count.txt"
[[ $(cat "$OUT/import-count.txt") -ge 5 ]]
pg_dump "$SRC" --role=auth_backup --format=custom --file="$FIX/templates.dump" 2> "$OUT/dump.stderr"
createdb -h 127.0.0.1 -p 55446 -U weaveos_test weaveos_template_restored
pg_restore --exit-on-error --dbname="$DST" "$FIX/templates.dump" > "$OUT/restore.txt" 2>&1
psql "$DST" -v ON_ERROR_STOP=1 -Atf "$OUT/snapshot.sql" > "$FIX/restored-template-snapshot.txt"
cmp "$FIX/source-template-snapshot.txt" "$FIX/restored-template-snapshot.txt"
sha256sum "$FIX/source-template-snapshot.txt" "$FIX/restored-template-snapshot.txt" > "$OUT/snapshot-sha256.txt"
psql "$DST" -v ON_ERROR_STOP=1 -Atc "SELECT count(*),bool_and(d.state='disabled' AND d.current_version=0 AND d.candidate_version=1) FROM applications.workflow_definitions d JOIN applications.operations o ON o.app_id=d.app_id AND o.operation_kind='application.template.import'; SELECT bool_and(NOT has_table_privilege('auth_app',c.oid,'INSERT') AND has_table_privilege('auth_backup',c.oid,'SELECT')) FROM applications.logical_tables t JOIN applications.operations o ON o.app_id=t.app_id AND o.operation_kind='application.template.import' JOIN pg_class c ON c.oid=to_regclass('appdata.t_'||replace(t.id::text,'-',''));" > "$OUT/restored-shape.txt"
set +e
goose -dir "$R/db/migrations" postgres "$DST" down > "$OUT/nonempty-down.txt" 2>&1;down=$?
set -e
[[ "$down" != 0 ]];grep -q 55000 "$OUT/nonempty-down.txt"
createdb -h 127.0.0.1 -p 55446 -U weaveos_test weaveos_template_empty
EMPTY='postgres://weaveos_test@127.0.0.1:55446/weaveos_template_empty?sslmode=disable'
goose -dir "$R/db/migrations" postgres "$EMPTY" up > "$OUT/empty-cycle.txt" 2>&1
goose -dir "$R/db/migrations" postgres "$EMPTY" down >> "$OUT/empty-cycle.txt" 2>&1
goose -dir "$R/db/migrations" postgres "$EMPTY" up >> "$OUT/empty-cycle.txt" 2>&1
psql "$DST" -Atc 'SELECT max(version_id) FROM goose_db_version WHERE is_applied' > "$OUT/post-down-version.txt"
[[ $(cat "$OUT/post-down-version.txt") == 32 ]]
date -u +%FT%TZ > "$OUT/finished.txt";echo 0 > "$OUT/exit.txt";cat "$OUT/import-count.txt" "$OUT/restored-shape.txt"
