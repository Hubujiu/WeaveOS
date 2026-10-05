#!/usr/bin/env bash
set -euo pipefail
rpc_root=/workspace/WeaveOS-worktrees/V030-032
rpc_name="weaveos-v032-regression-$(date +%s)-$$"
rpc_network="${rpc_name}-network"
rpc_pg="${rpc_name}-postgres"
rpc_runner="${rpc_name}-runner"
cleanup(){ docker rm -f -v "$rpc_runner" "$rpc_pg" >/dev/null 2>&1 || true; docker network rm "$rpc_network" >/dev/null 2>&1 || true; }
trap cleanup EXIT
docker network create --internal --label weaveos.package=V030-032 "$rpc_network" >/dev/null
docker run -d --name "$rpc_pg" --network "$rpc_network" --network-alias b3-postgres --label weaveos.package=V030-032 -e POSTGRES_DB=b3_flowable_fixture -e POSTGRES_USER=b3_fixture -e POSTGRES_PASSWORD=b3_fixture_only --tmpfs /var/lib/postgresql:rw 'docker.io/library/postgres@sha256:5a5a84b19854a9ffaa54082c166ff4ec27473a361e496e5ea167f298f2da9722' >/dev/null
for rpc_attempt in {1..60}; do
 if docker exec "$rpc_pg" pg_isready -U b3_fixture -d b3_flowable_fixture >/dev/null 2>&1; then break; fi
 if [[ "$rpc_attempt" == 60 ]]; then docker logs "$rpc_pg"; exit 1; fi
 sleep 1
done
docker network inspect --format 'fixture network internal={{.Internal}}' "$rpc_network"
docker inspect --format 'fixture published ports={{json .HostConfig.PortBindings}}' "$rpc_pg"
docker run --rm --name "$rpc_runner" --user "$(id -u):$(id -g)" --network "$rpc_network" --label weaveos.package=V030-032 -v "$rpc_root:/repo:ro" -w /repo/services/bff/internal/flowcommands -e WEAVEOS_TEST_DATABASE_URL='postgres://b3_fixture:b3_fixture_only@b3-postgres:5432/b3_flowable_fixture?sslmode=disable' --entrypoint /repo/services/workflow-engine/.work/flowcommands-regression.test 'mirror.gcr.io/library/maven@sha256:fa7aa19829157d299ff05f631b51697a388dcd2f6955e84249ecc652015f217b' -test.v=test2json -test.timeout=180s | /workspace/.weaveos-tools/go/bin/go tool test2json -t -p github.com/Hubujiu/WeaveOS/services/bff/internal/flowcommands | tee "$rpc_root/docs/evidence/V030-032/execution-rpc-green/flowcommands-fixture.jsonl"
