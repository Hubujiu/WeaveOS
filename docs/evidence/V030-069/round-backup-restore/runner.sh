#!/bin/bash
set -euo pipefail
ROOT=/workspace/scratch/75c2273f6a80
R=$ROOT/WeaveOS-worktrees/V030-069
FIX=$ROOT/app-recovery-fixtures/V030-069-round-formal-third
OUT=$R/docs/evidence/V030-069/round-backup-restore
PG=$ROOT/restored-tools/pgdist/usr/lib/postgresql/18/bin
export LD_LIBRARY_PATH=$ROOT/restored-tools/pgdist/usr/lib/x86_64-linux-gnu
export PATH="$PG:$ROOT/restored-tools/gopath/bin:$PATH"
[[ ! -e "$OUT" ]] || exit 2
mkdir "$OUT"; cp "$0" "$OUT/runner.sh"
git -C "$R" rev-parse HEAD > "$OUT/source-head.txt"
"$PG/pg_ctl" -D "$FIX/pgdata" -l "$FIX/backup.log" -o "-h 127.0.0.1 -p 55446 -k ''" start >/dev/null
trap '"$PG/pg_ctl" -D "$FIX/pgdata" stop -m fast >/dev/null' EXIT
SRC='postgres://weaveos_test@127.0.0.1:55446/weaveos_ci_test?sslmode=disable'
DST='postgres://weaveos_test@127.0.0.1:55446/weaveos_round_restored?sslmode=disable'
date -u +%FT%TZ > "$OUT/started.txt"
SQL="COPY (SELECT row_to_json(r)::text FROM applications.workflow_rounds r ORDER BY instance_id) TO STDOUT"
psql "$SRC" -v ON_ERROR_STOP=1 -c "$SQL" > "$FIX/source-rounds.jsonl"
[[ $(wc -l < "$FIX/source-rounds.jsonl") -ge 6 ]]
pg_dump "$SRC" --role=auth_backup --format=custom --file="$FIX/rounds.dump" 2> "$OUT/dump.stderr"
createdb -h 127.0.0.1 -p 55446 -U weaveos_test weaveos_round_restored
pg_restore --exit-on-error --dbname="$DST" "$FIX/rounds.dump" > "$OUT/restore.txt" 2>&1
psql "$DST" -v ON_ERROR_STOP=1 -c "$SQL" > "$FIX/restored-rounds.jsonl"
cmp "$FIX/source-rounds.jsonl" "$FIX/restored-rounds.jsonl"
sha256sum "$FIX/source-rounds.jsonl" "$FIX/restored-rounds.jsonl" > "$OUT/identity-sha256.txt"
psql "$DST" -v ON_ERROR_STOP=1 -Atc "SELECT round_kind,count(*) FROM applications.workflow_rounds GROUP BY round_kind ORDER BY round_kind; SELECT has_table_privilege('auth_app','applications.workflow_rounds','SELECT'),has_table_privilege('auth_app','applications.workflow_rounds','INSERT'),has_table_privilege('auth_app','applications.workflow_rounds','UPDATE'),has_table_privilege('auth_app','applications.workflow_rounds','DELETE'),has_table_privilege('auth_backup','applications.workflow_rounds','SELECT'); SELECT tgname FROM pg_trigger WHERE tgrelid='applications.workflow_rounds'::regclass AND NOT tgisinternal ORDER BY tgname;" > "$OUT/restored-shape.txt"
set +e
goose -dir "$R/db/migrations" postgres "$SRC" down > "$OUT/nonempty-down.txt" 2>&1;down=$?
set -e
[[ "$down" != 0 ]];grep -q 55000 "$OUT/nonempty-down.txt"
createdb -h 127.0.0.1 -p 55446 -U weaveos_test weaveos_round_empty
EMPTY='postgres://weaveos_test@127.0.0.1:55446/weaveos_round_empty?sslmode=disable'
goose -dir "$R/db/migrations" postgres "$EMPTY" up > "$OUT/empty-cycle.txt" 2>&1
goose -dir "$R/db/migrations" postgres "$EMPTY" down >> "$OUT/empty-cycle.txt" 2>&1
goose -dir "$R/db/migrations" postgres "$EMPTY" down >> "$OUT/empty-cycle.txt" 2>&1
goose -dir "$R/db/migrations" postgres "$EMPTY" up >> "$OUT/empty-cycle.txt" 2>&1
psql "$SRC" -Atc 'SELECT max(version_id) FROM goose_db_version WHERE is_applied' > "$OUT/post-down-version.txt"
date -u +%FT%TZ > "$OUT/finished.txt";echo 0 > "$OUT/exit.txt";cat "$OUT/restored-shape.txt"
