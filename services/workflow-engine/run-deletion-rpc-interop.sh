#!/usr/bin/env bash
set -euo pipefail
rpc_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
rpc_root=$(cd "$rpc_dir/../.." && pwd)
rpc_go=${WEAVEOS_RPC_GO:-go}
test "$($rpc_go version | cut -d ' ' -f 3)" = go1.27.2
rpc_name="weaveos-v067-$(date +%s)-$$"
rpc_network="${rpc_name}-network"
rpc_pg="${rpc_name}-postgres"
rpc_java="${rpc_name}-java"
rpc_maven_image='mirror.gcr.io/library/maven@sha256:fa7aa19829157d299ff05f631b51697a388dcd2f6955e84249ecc652015f217b'
rpc_pg_image='docker.io/library/postgres@sha256:5a5a84b19854a9ffaa54082c166ff4ec27473a361e496e5ea167f298f2da9722'
mkdir -p "$rpc_dir/.work"
cleanup() {
  if docker inspect "$rpc_java" >/dev/null 2>&1; then
    docker stop --time 15 "$rpc_java" >/dev/null 2>&1 || true
    docker logs "$rpc_java" 2>&1 || true
    docker rm "$rpc_java" >/dev/null 2>&1 || true
  fi
  docker rm -f -v "$rpc_pg" >/dev/null 2>&1 || true
  docker network rm "$rpc_network" >/dev/null 2>&1 || true
}
trap cleanup EXIT
# Compile the existing tagged Root test to a static Linux executable with the locked Go SDK.
# No host RPC connection is made; execution below is inside the private fixture network.
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=local "$rpc_go" -C "$rpc_root/services/bff" test \
  -tags workflow_deletion_integration -c -o "$rpc_dir/.work/deletion-interop.test" ./internal/workflowrpc
docker network create --internal --label weaveos.package=V030-067 "$rpc_network" >/dev/null
docker run -d --name "$rpc_pg" --label weaveos.package=V030-067 --network "$rpc_network" --network-alias b3-postgres \
  -e POSTGRES_DB=b3_flowable_fixture -e POSTGRES_USER=b3_fixture -e POSTGRES_PASSWORD=b3_fixture_only \
  --tmpfs /var/lib/postgresql:rw "$rpc_pg_image" >/dev/null
for rpc_attempt in {1..60}; do
  if docker exec "$rpc_pg" pg_isready -U b3_fixture -d b3_flowable_fixture >/dev/null 2>&1; then break; fi
  if [[ "$rpc_attempt" == 60 ]]; then docker logs "$rpc_pg"; exit 1; fi
  sleep 1
done
docker network inspect --format 'fixture network internal={{.Internal}}' "$rpc_network"
docker inspect --format 'fixture published ports={{json .HostConfig.PortBindings}}' "$rpc_pg"
docker run --rm --user "$(id -u):$(id -g)" --network "$rpc_network" --label weaveos.package=V030-067 \
  -v "$rpc_dir:/proof" -v "$rpc_dir/.work/m2:/m2" -w /proof --entrypoint mvn "$rpc_maven_image" \
  -B -ntp -o -s maven-settings.xml -Duser.home=/tmp -Dmaven.repo.local=/m2 test-compile \
  org.apache.maven.plugins:maven-dependency-plugin:3.9.0:build-classpath -Dmdep.outputFile=.work/rpc-classpath -Dmdep.includeScope=test
docker run -d --name "$rpc_java" --user "$(id -u):$(id -g)" --network "$rpc_network" --network-alias b3-workflow \
  --label weaveos.package=V030-067 -v "$rpc_dir:/proof" -v "$rpc_dir/.work/m2:/m2:ro" -w /proof \
  -e B3_TEST_JDBC_URL=jdbc:postgresql://b3-postgres:5432/b3_flowable_fixture --entrypoint sh "$rpc_maven_image" \
  -ec 'exec java -cp "target/classes:target/test-classes:$(cat .work/rpc-classpath)" org.weaveos.workflow.RootFlowDeletionInteropFixtureMain' >/dev/null
for rpc_attempt in {1..60}; do
  if docker exec "$rpc_java" test -f /tmp/weaveos-v067-deletion-ready; then break; fi
  if [[ "$rpc_attempt" == 60 ]]; then docker logs "$rpc_java"; exit 1; fi
  sleep 1
done
rpc_port=$(docker exec "$rpc_java" cat /tmp/weaveos-v067-deletion-ready)
[[ "$rpc_port" =~ ^[0-9]+$ ]]
docker inspect --format 'fixture published ports={{json .HostConfig.PortBindings}}' "$rpc_java"
rpc_execute() {
  docker exec -e NO_PROXY=b3-workflow,b3-postgres,127.0.0.1,localhost -e no_proxy=b3-workflow,b3-postgres,127.0.0.1,localhost \
    -e WEAVEOS_V067_RPC_TARGET="127.0.0.1:$rpc_port" "$rpc_java" \
    /proof/.work/deletion-interop.test -test.v=test2json -test.run '^TestRootGoJavaActualDeletionRuntimeInterop$' -test.timeout=90s
}
if [[ -n "${WEAVEOS_RPC_JSON_REPORT:-}" ]]; then
  mkdir -p -- "$(dirname -- "$WEAVEOS_RPC_JSON_REPORT")"
  # Only genuine test binary stdout enters test2json. Maven/fixture logs stay outside.
  # set -o pipefail preserves failure of the original test, converter or report writer.
  rpc_execute | "$rpc_go" tool test2json -t -p github.com/Hubujiu/WeaveOS/services/bff/internal/workflowrpc | tee "$WEAVEOS_RPC_JSON_REPORT"
else
  rpc_execute
fi
