#!/bin/bash
# Supplemental real logical restore verification of existing backup privileges.
# No encrypted/Docker backup claim. Input: stopped isolated regression fixture.
set -euo pipefail
ROOT=${1:?workspace tools root}; FIX=${2:?stopped isolated PGDATA}; OUT=${3:?evidence directory}
PG=$ROOT/restored-tools/pg/bin
TMP=$(mktemp -d "$ROOT/v044-backup.XXXXXX")
"$PG/pg_ctl" -D "$FIX" -l "$TMP/postgres.log" -o "-h 127.0.0.1 -p 55445 -k ''" start >/dev/null
trap '"$PG/pg_ctl" -D "$FIX" stop -m fast >/dev/null' EXIT
SRC='postgres://weaveos_test@127.0.0.1:55445/weaveos_v044?sslmode=disable'
DST='postgres://weaveos_test@127.0.0.1:55445/weaveos_trigger_restore?sslmode=disable'
"$PG/createdb" -h 127.0.0.1 -p 55445 -U weaveos_test weaveos_trigger_restore
QUERY="SELECT app_id,flow_id,version,triggers_json FROM applications.workflow_versions ORDER BY app_id,flow_id,version"
"$PG/psql" "$SRC" -At -v ON_ERROR_STOP=1 -c "$QUERY" > "$TMP/before.txt"
"$PG/psql" "$SRC" -At -v ON_ERROR_STOP=1 -c "SELECT count(*) FILTER(WHERE triggers_json='[]'::jsonb),count(*) FILTER(WHERE triggers_json<>'[]'::jsonb) FROM applications.workflow_versions" > "$OUT/row-counts.txt"
python - "$OUT/row-counts.txt" <<'PY'
import sys
empty,configured=map(int,open(sys.argv[1]).read().strip().split('|'))
assert empty>0 and configured>0, 'must cover both legacy-empty and nonempty configurations'
PY
"$PG/pg_dump" "$SRC" --role=auth_backup --format=custom --file="$TMP/backup.dump"
"$PG/pg_restore" --dbname="$DST" --no-owner --no-privileges --exit-on-error "$TMP/backup.dump"
"$PG/psql" "$DST" -At -v ON_ERROR_STOP=1 -c "$QUERY" > "$TMP/after.txt"
cmp "$TMP/before.txt" "$TMP/after.txt"
sha256sum "$TMP/before.txt" "$TMP/after.txt" | sed "s|$TMP/||" > "$OUT/row-digests.txt"
if "$PG/psql" "$SRC" -v ON_ERROR_STOP=1 -c "SET ROLE auth_backup; UPDATE applications.workflow_versions SET triggers_json='[]' WHERE false;" > "$OUT/write-denial.txt" 2>&1; then
 echo 'ERROR: backup role may mutate immutable versions'; exit 1
fi
grep -q 'permission denied for table workflow_versions' "$OUT/write-denial.txt"
echo 'PASS: restricted logical dump/restore preserves every version and trigger value; backup UPDATE denied.'
