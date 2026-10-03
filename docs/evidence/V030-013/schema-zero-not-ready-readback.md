# Authorized initial schema0 real HTTPS probe — owner handoff

Source reread: accepted V015 ADR https://app.notion.com/p/3ee2f5a9e6488163a144fba5f55e3c14,
page_last_edited_at 2026-10-03T14:47:12.842Z, unchanged from recorded freeze.
ADR8.5: schemaReady=false returns409 APPLICATION_SCHEMA_NOT_READY.
Public Version accepts0; existing shared body decoder positively tests schema0.

Immutable consumed owner source:54316eaee397eeafd0546842e3b90be1e3cb2e5c.
Probe baseline:b6616ae36de829bbd458294da7c395f1e6b7dd5b.
This independent source overlay inserts one expectation before first definition
Save in the real certificate-verified HTTPS BFF fixture. Actual registered form
and owner Session, real Redis/PG18/auth_app; no product or V015 source edit.
It asserts POSTrecords expectedSchemaVersion0 returns409 SCHEMA_NOT_READY.
Observed target RED:400 COMMON_VALIDATION_FAILED (schema guard rejects0 before
actual resource/action/readiness check). Remaining fixture runs against real
schema1 and unchanged record/draft/runtime/history/candidate expectations.

Exact probe source:schema-zero-not-ready-http-probe.go.
Overlay mapping:services/bff/cmd/bff/definition_wiring_test.go -> that probe;
local /tmp/v013-schema-zero-overlay.json contains absolute paths to both.
Command: with taskRecordEnv (same PG/Redis/cache environment in existing full
regression), /workspace/.weaveos-tools/go/bin/go test -race -p1 -count=1
-overlay=/tmp/v013-schema-zero-overlay.json ./cmd/bff
-run TestBFFCompositionExposesDefinitionWithSameSession.
Actual exit1, exact output:schema-zero-not-ready-http-probe-red.txt. No skip.
This probe is separate known failure, not a passing regular suite expectation.

Concrete unapplied owner-only repair:proposed-v015-schema-zero.patch changes
only Create/Edit negative-schema input guard from<1 to<0, preserving existing
same-Tx authorization/readiness and minimum replay. The separate schema999
conflict probe/patch remain outstanding. V015 owner must test/fix and return a
lead-accepted immutable source; this proposal is neither applied nor GREEN.
