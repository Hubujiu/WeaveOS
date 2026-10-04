#!/usr/bin/env bash
set -euo pipefail
proof_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
proof_name="weaveos-v022-$(date +%s)-$$"
proof_network="${proof_name}-network"
proof_db="${proof_name}-postgres"
proof_maven_image='mirror.gcr.io/library/maven@sha256:fa7aa19829157d299ff05f631b51697a388dcd2f6955e84249ecc652015f217b'
proof_pg_image='docker.io/library/postgres@sha256:5a5a84b19854a9ffaa54082c166ff4ec27473a361e496e5ea167f298f2da9722'
mkdir -p "$proof_dir/.work/m2"
cleanup() {
  docker rm -f "${proof_name}-maven" >/dev/null 2>&1 || true
  docker rm -f -v "$proof_db" >/dev/null 2>&1 || true
  docker network rm "$proof_network" >/dev/null 2>&1 || true
}
trap cleanup EXIT
docker network create --internal --label weaveos.package=V030-022 "$proof_network" >/dev/null
docker run -d --name "$proof_db" --label weaveos.package=V030-022 --network "$proof_network" --network-alias b3-postgres \
  -e POSTGRES_DB=b3_flowable_fixture -e POSTGRES_USER=b3_fixture -e POSTGRES_PASSWORD=b3_fixture_only \
  --tmpfs /var/lib/postgresql:rw "$proof_pg_image" >/dev/null
for proof_attempt in {1..60}; do
  if docker exec "$proof_db" pg_isready -U b3_fixture -d b3_flowable_fixture >/dev/null 2>&1; then break; fi
  if [[ "$proof_attempt" == 60 ]]; then docker logs "$proof_db"; exit 1; fi
  sleep 1
done
docker exec "$proof_db" postgres --version
docker inspect --format 'fixture published ports={{json .HostConfig.PortBindings}}' "$proof_db"
docker network inspect --format 'fixture network internal={{.Internal}}' "$proof_network"
# Only the private fixture network, no published ports or mounted Docker socket.
docker run --rm --name "${proof_name}-maven" --user "$(id -u):$(id -g)" --network "$proof_network" --label weaveos.package=V030-022 \
  -v "$proof_dir:/proof" -v "$proof_dir/.work/m2:/m2" -w /proof \
  -e B3_TEST_JDBC_URL=jdbc:postgresql://b3-postgres:5432/b3_flowable_fixture \
  --entrypoint mvn "$proof_maven_image" -B -ntp -s maven-settings.xml -Duser.home=/tmp -Dmaven.repo.local=/m2 -o "$@"
