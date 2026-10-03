# Root schema-boundary regression and service fix

The test-only baseline is commit `5018e05b0304ef341f1d9cc0d4619b78b65fbfba`,
whose parent is `54316eaee397eeafd0546842e3b90be1e3cb2e5c`. The isolated worktree
was clean at that baseline before the one-file implementation change. The
Root-authored `schema_boundary_root_test.go` SHA-256 was
`359ad1d354bbcfb1be96fc3fbe766cebb7216c8e6dd2140ecbdeea7b37034b6b` before
and after implementation.

## R01: independent RED

The actual command used the `TestRootSchema` prefix selector below. It ran in
`services/bff` with the existing isolated PostgreSQL and Redis settings. The
R01 work-order file was not locally available to the runner, so this records the
executed prefix command and does not claim it was byte-for-byte the command in
that work order.

```sh
set -o pipefail
WEAVEOS_TEST_DATABASE_URL='postgres://weaveos_owner@127.0.0.1:65432/weaveos_v015_consumer?sslmode=disable' \
WEAVEOS_TEST_REDIS_URL='redis://127.0.0.1:65433/14' \
GOCACHE=/tmp/weaveos-v015-go-cache GOPATH=/workspace/.weaveos-tools/gopath \
/workspace/.weaveos-tools/go/bin/go test -race ./internal/apprecordservice \
  -run '^TestRootSchema' -count=1 -v 2>&1 \
  | tee /tmp/weaveos-r01-library/schema-boundary-root-test.log
```

Exit status: `1`. All seven subcases reached their target assertions:

- Four schema-zero cases (create/edit, authorized/menu-only) failed with
  `invalid record input`.
- Two schema-mismatch cases (create/edit) failed with `record version conflict`.
- `stale-record-with-current-schema` passed, retaining record-CAS identity.

This was a real isolated PostgreSQL/Redis RED, not a compile or environment
failure. The unmodified log is [r01-prefix-red.log](r01-prefix-red.log),
SHA-256 `1c81af2345e4f8425ebbf4947f7f2151703962160225d43c64f0c4e950cbc720`.

## R02: four-place service correction

Only `services/bff/internal/apprecordservice/write.go` changed:

1. Create accepts `expectedSchemaVersion == 0` through input validation.
2. Edit accepts `expectedSchemaVersion == 0`; its
   `expectedRecordVersion < 1` validation remains unchanged.
3. Create schema-version mismatch returns
   `APPLICATION_SCHEMA_CONFLICT` with `currentSchemaVersion`.
4. Edit schema-version mismatch returns the same structured schema error.

The change leaves the live-authorization/not-ready order, record CAS,
replay, source, and transaction paths untouched. No test, fixture, writer, or
HTTP code was changed.

Targeted command, exit status `0`:

```sh
set -o pipefail
WEAVEOS_TEST_DATABASE_URL='postgres://weaveos_owner@127.0.0.1:65432/weaveos_v015_consumer?sslmode=disable' \
WEAVEOS_TEST_REDIS_URL='redis://127.0.0.1:65433/14' \
GOCACHE=/tmp/weaveos-v015-go-cache GOPATH=/workspace/.weaveos-tools/gopath \
/workspace/.weaveos-tools/go/bin/go test -race -count=1 -timeout 2m \
  ./internal/apprecordservice \
  -run '^TestRoot(SchemaZeroUsesLiveAuthorizationBeforeNotReady|SchemaConflictIsDistinctFromRecordConflict)$' \
  -v 2>&1 | tee /tmp/weaveos-r01-library/r02-targeted.log
```

All seven Root subcases passed. Captured output is
[r02-root-targeted-green.log](r02-root-targeted-green.log), SHA-256
`e99aee76d80d05625973753b973c523cac016333e6c8d07bf2790c0dc31e056c`.

Existing service-package race regression, exit status `0` (5.158s):

```sh
set -o pipefail
WEAVEOS_TEST_DATABASE_URL='postgres://weaveos_owner@127.0.0.1:65432/weaveos_v015_consumer?sslmode=disable' \
WEAVEOS_TEST_REDIS_URL='redis://127.0.0.1:65433/14' \
GOCACHE=/tmp/weaveos-v015-go-cache GOPATH=/workspace/.weaveos-tools/gopath \
/workspace/.weaveos-tools/go/bin/go test -race -count=1 -timeout 2m \
  ./internal/apprecordservice 2>&1 \
  | tee /tmp/weaveos-r01-library/r02-package.log
```

Captured output is
[r02-apprecordservice-package-green.log](r02-apprecordservice-package-green.log),
SHA-256 `cd40c8875c4903b3f9af72864ae93dec67f6f129d210b2297e7b58e3875b4bb3`.

This evidence establishes service-level behavior against isolated PostgreSQL
and Redis. It does not establish V013 HTTPS/API or browser integration.
