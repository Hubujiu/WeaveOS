#!/usr/bin/env bash
# Root-owned additional integration acceptance, not a production migration command.
set -euo pipefail
proof_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
goose=$(realpath -- "${WEAVEOS_SCHEMA_TEST_GOOSE:?explicit test Goose binary required}")
test -x "$goose"
name="weaveos-v041-schema-$(date +%s)-$$"
network="$name-network"
db="$name-postgres"
pg_image='docker.io/library/postgres@sha256:5a5a84b19854a9ffaa54082c166ff4ec27473a361e496e5ea167f298f2da9722'
maven_image='mirror.gcr.io/library/maven@sha256:fa7aa19829157d299ff05f631b51697a388dcd2f6955e84249ecc652015f217b'
mkdir -p "$proof_dir/.work" "$proof_dir/schema-cli-reports"
work=$(mktemp -d "$proof_dir/.work/schema-cli.XXXXXX")
report="$proof_dir/schema-cli-reports/cases.txt"
: > "$report"
cleanup(){
 docker rm -f "$name-cli" >/dev/null 2>&1 || true
 docker rm -f -v "$db" >/dev/null 2>&1 || true
 docker network rm "$network" >/dev/null 2>&1 || true
 rm -rf -- "$work"
}
trap cleanup EXIT
pass(){ printf '%s\n' "$1" | tee -a "$report"; }
owner(){ docker exec -i "$db" psql -X -v ON_ERROR_STOP=1 -U b3_fixture -d b3_flowable_fixture "$@"; }
query(){ owner -tAc "$1"; }
runtime(){ docker exec -i -e PGPASSWORD=v041_synthetic_only "$db" psql -X -v ON_ERROR_STOP=1 -v VERBOSITY=verbose -h 127.0.0.1 -U v041_runtime -d b3_flowable_fixture "$@"; }
dsn='postgres://b3_fixture:b3_fixture_only@b3-postgres:5432/b3_flowable_fixture?sslmode=disable&search_path=workflow'
migrate(){
 docker run --rm --name "$name-cli" --user "$(id -u):$(id -g)" --network "$network" \
  -v "$goose:/goose:ro" -v "$work:/migrations:ro" --entrypoint /goose "$maven_image" \
  -dir /migrations -table workflow.goose_db_version postgres "$dsn" "$@"
}
# Images are fetched before the test's no-egress network is used.
docker pull "$pg_image" >/dev/null
docker pull "$maven_image" >/dev/null
docker network create --internal --label weaveos.package=V030-041 "$network" >/dev/null
docker run -d --name "$db" --network "$network" --network-alias b3-postgres \
 --label weaveos.package=V030-041 --tmpfs /var/lib/postgresql:rw \
 -e POSTGRES_DB=b3_flowable_fixture -e POSTGRES_USER=b3_fixture -e POSTGRES_PASSWORD=b3_fixture_only "$pg_image" >/dev/null
ready=0
for attempt in {1..60}; do
 if docker exec "$db" pg_isready -U b3_fixture -d b3_flowable_fixture >/dev/null 2>&1; then ready=1; break; fi
 sleep 1
done
test "$ready" = 1
test "$(docker network inspect --format '{{.Internal}}' "$network")" = true
test "$(docker inspect --format '{{json .HostConfig.PortBindings}}' "$db")" = '{}'
pass isolated_network_without_published_ports
owner -c 'CREATE SCHEMA workflow AUTHORIZATION b3_fixture;' >/dev/null
cp "$proof_dir/schema/migrations/00001_flowable8.sql" "$work/"
migrate up
test "$(query "SELECT count(*) FROM pg_tables WHERE schemaname='workflow' AND (tablename LIKE 'act_%' OR tablename LIKE 'flw_%')")" = 32
test "$(query "SELECT count(*) FROM pg_tables WHERE schemaname='workflow' AND tablename LIKE 'wf_%'")" = 4
test "$(query 'SELECT max(version_id) FROM workflow.goose_db_version WHERE is_applied')" = 1
pass explicit_goose_up_installs_native_and_ledger_schema
owner -c "INSERT INTO workflow.act_hi_procinst(id_,proc_inst_id_,proc_def_id_,start_time_) VALUES ('retained-history','retained-history','synthetic-definition',TIMESTAMP '2026-10-06 00:00:00');" >/dev/null
migrate up
test "$(query 'SELECT count(*) FROM workflow.goose_db_version WHERE is_applied AND version_id=1')" = 1
test "$(query "SELECT count(*) FROM workflow.act_hi_procinst WHERE id_='retained-history'")" = 1
pass repeated_up_keeps_version_and_history
cat > "$work/00002_failure.sql" <<'SQL'
-- +goose Up
CREATE TABLE must_not_survive (id integer);
SELECT 1/0;
-- +goose Down
DROP TABLE must_not_survive;
SQL
if migrate up > "$work/failed-up.log" 2>&1; then echo 'expected failing second migration' >&2; exit 1; fi
grep -q 'division by zero' "$work/failed-up.log"
test "$(query "SELECT to_regclass('workflow.must_not_survive') IS NULL")" = t
test "$(query 'SELECT max(version_id) FROM workflow.goose_db_version WHERE is_applied')" = 1
pass failed_up_rolls_back_ddl_and_version
rm -- "$work/00002_failure.sql"
if migrate down > "$work/down.log" 2>&1; then echo 'expected protected Down failure' >&2; exit 1; fi
grep -q 'workflow schema downgrade is not supported' "$work/down.log"
test "$(query "SELECT count(*) FROM workflow.act_hi_procinst WHERE id_='retained-history'")" = 1
test "$(query 'SELECT max(version_id) FROM workflow.goose_db_version WHERE is_applied')" = 1
pass down_refuses_history_deletion
# Synthetic role lives only in this disposable database container.
owner <<'SQL'
CREATE ROLE v041_runtime LOGIN PASSWORD 'v041_synthetic_only' NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
REVOKE TEMPORARY ON DATABASE b3_flowable_fixture FROM PUBLIC;
GRANT USAGE ON SCHEMA workflow TO v041_runtime;
SELECT format('GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE %I.%I TO v041_runtime',schemaname,tablename)
 FROM pg_tables WHERE schemaname='workflow' AND (tablename LIKE 'act_%' OR tablename LIKE 'flw_%') \gexec
SELECT format('GRANT USAGE, SELECT ON SEQUENCE %I.%I TO v041_runtime',schemaname,sequencename)
 FROM pg_sequences WHERE schemaname='workflow' AND (sequencename LIKE 'act_%' OR sequencename LIKE 'flw_%') \gexec
GRANT SELECT,INSERT ON workflow.wf_deployments,workflow.wf_execution_commands,workflow.wf_execution_instances,workflow.wf_execution_tasks TO v041_runtime;
GRANT UPDATE(status,engine_deployment_id,process_definition_id) ON workflow.wf_deployments TO v041_runtime;
GRANT UPDATE(outcome,result_sequence,proof_id,result_hash,result_bytes) ON workflow.wf_execution_commands TO v041_runtime;
GRANT UPDATE(engine_process_id,state,sequence,fence_epoch,schema_version,record_version,activation_epoch,updated_at) ON workflow.wf_execution_instances TO v041_runtime;
GRANT UPDATE(state,decision) ON workflow.wf_execution_tasks TO v041_runtime;
SQL
runtime -c "SELECT value_ FROM workflow.act_ge_property WHERE name_='schema.version'; INSERT INTO workflow.act_ge_property(name_,value_,rev_) VALUES ('synthetic-runtime-probe','probe',1); DELETE FROM workflow.act_ge_property WHERE name_='synthetic-runtime-probe'; INSERT INTO workflow.wf_deployments(version_id,app_id,flow_id,version,bpmn_sha256,status) VALUES ('00000001-0000-4000-8000-000000000001','00000002-0000-4000-8000-000000000002','00000003-0000-4000-8000-000000000003',1,repeat('0',64),'pending'); UPDATE workflow.wf_deployments SET status='confirmed',engine_deployment_id='synthetic',process_definition_id='synthetic';" >/dev/null
pass runtime_can_use_authorized_native_and_ledger_operations
for sql in 'CREATE TABLE workflow.forbidden(id integer)' 'DROP TABLE workflow.act_hi_procinst' 'TRUNCATE workflow.act_hi_procinst' 'CREATE SCHEMA forbidden' 'CREATE TABLE public.forbidden(id integer)' 'CREATE TEMPORARY TABLE forbidden(id integer)' 'CREATE ROLE forbidden'; do
 if runtime -c "$sql" > "$work/denied.log" 2>&1; then echo 'runtime DDL unexpectedly allowed' >&2; exit 1; fi
 grep -q '42501' "$work/denied.log"
done
pass runtime_ddl_and_truncate_are_denied
for sql in 'DELETE FROM workflow.wf_deployments' "UPDATE workflow.wf_deployments SET bpmn_sha256=repeat('1',64)" 'SELECT * FROM workflow.goose_db_version'; do
 if runtime -c "$sql" > "$work/denied.log" 2>&1; then echo 'runtime immutable ledger or migration access unexpectedly allowed' >&2; exit 1; fi
 grep -q '42501' "$work/denied.log"
done
test "$(query "SELECT count(*) FROM workflow.act_hi_procinst WHERE id_='retained-history'")" = 1
pass runtime_cannot_delete_or_rewrite_protocol_identity
python3 - "$report" <<'PY'
from pathlib import Path
import sys
expected=['isolated_network_without_published_ports','explicit_goose_up_installs_native_and_ledger_schema','repeated_up_keeps_version_and_history','failed_up_rolls_back_ddl_and_version','down_refuses_history_deletion','runtime_can_use_authorized_native_and_ledger_operations','runtime_ddl_and_truncate_are_denied','runtime_cannot_delete_or_rewrite_protocol_identity']
assert Path(sys.argv[1]).read_text().splitlines()==expected
print('ROOT_SCHEMA_CLI: 8 actual cases passed')
PY
