# V030-071 · Root delivery review (in progress)

This is a live review, not a completion or promotion declaration. Root writes and tests directly in its own environment. No delegated executor, main merge, or deployment.

## Sources and product boundary

- [V071 PRD](https://app.notion.com/p/3f52f5a9e64881c79883f48641f1ac8e)
- [V071 ADR](https://app.notion.com/p/3f52f5a9e648819ab755f3b1134e78af)
- [V071 data contract](https://app.notion.com/p/3f52f5a9e648817ab711c73ba95300bc)
- Accepted V015 private per-view presets and V013 real-table/history rules are the normative predecessors.
- Record/container copy and delete candidates remain unanswered; this work does not implement those policies. Discarding an explicitly selected personal preset is a different already specified operation.

## Reviewed implementation boundaries

- Current Session actor is the owner; application + actual form-view scopes are server resolved. Same-table views and different users never share private configuration.
- Existing record policy resolution is factored without changing its authorization decisions. Criteria/sort use whole-readable-row coverage; display-only fields use readable field scope. Hidden columns never narrow the authorized business projection.
- Each private view has 20 slots, case-sensitive unique trimmed names, and optimistic versions. Existing application transaction locking serializes allocation. No table/record/query revision, audit, fence, or Flowable side effect.
- Raw JSON has 64 KiB, filter 16 KiB and canonical saved state 32 KiB independent budgets. Duplicate keys, invalid UTF-8/surrogates, extra keys, arbitrary system columns and unsupported trees are rejected. The typed query compiler remains authoritative.
- Stored reference kinds detect deletion/type changes. Permission changes invalidate the entire configuration. Invalid DTOs retain exactly id/name/version/invalid/reason; original settings are retained in storage but never leaked through the invalid DTO.
- Original confirmed mutations recover only operationId/id/version. Replay precedes current schema/permission/CAS reevaluation. New writes always reevaluate current authorization. Generic recovery remains actor scoped.
- Actual COMMIT acknowledgement loss returns 503 with operationId only. Confirmed transaction plus session revocation remains successful and clears cookies. Late write errors roll back configuration and operation together.
- HTTP POST/GET/PUT/DELETE are exercised through the real composed HTTPS BFF. HEAD/bodyless-delete contract and strict identity/CSRF guards remain closed.
- Public generic operation recovery now includes the four previously omitted approved workflow DTOs. Only existing closed schema references are appended; runtime authorization and old DTO properties remain unchanged.

## Evidence so far (not formal migration certification)

- Configuration RED → GREEN: 6 top-level +43 subtests; shared policy fact extraction is separately tested.
- Old develop hot31 + explicitly isolated fixture SQL: 12 top-level +30 subtests passed, including concurrency, CAS, isolation, invalidation, real HTTP, injected storage failures and genuine COMMIT/Redis faults.
- Real captured POST/GET response bodies pass the public route schemas using standard Ajv2020. Legacy schema helper cannot interpret `not`; its error is retained, no constraints were removed.
- Actual isolated auth_backup dump and new-database restore preserve all 32 configurations and 49 minimum receipts byte-for-byte, along with original structure/permissions and personnel presets. No business/user data was used.
- Four existing workflow recovery variants: six independent public-route examples RED before implementation; afterward all 551 Node contract/governance/foundation tests pass, no skips. OpenAPI valid with 14 warnings.
- Development broad regression revealed missing formal role registration and two obsolete explicit fixture cleanup lists. It was also not source-frozen and is never treated as final acceptance. All original failures retained.
- Explicit cleanup lists now include the new FK child without CASCADE or relaxed assertions; applications + audit race regression: 50 top-level +12 subtests passed.
- Formal role reapplication is an intentional remaining RED until production migration/roles are installed. Twelve other top-level +30 subtests remain passing in that run.
- Full Go vet/build and formatting check passed before dependency integration.

## Required before ready

1. Verify PR79 exact b5872f40 final CI/product evidence, squash develop under the existing authorization, and integrate the actual result. Preserve TemplateImportResult and all preset/workflow recovery branches.
2. Freeze the available official hot migration number in the data source, then implement forward schema, minimal roles and protected Down. The current isolated SQL is not a production migration.
3. Verify real forward/empty-down/re-up; reject Down with private configurations or any of the three operation histories, retaining rows and Goose version. Preserve all preceding operation kinds/constraints.
4. Update exact migration and role compatibility hashes, backup shape expectations, and keep backwardCompatible=false. No main promotion or deployment.
5. Repeat real storage, HTTPS, backup/restore, all affected Go race packages, vet/build, Node checks and secret scan on frozen integrated source. Review any failure without weakening expectations.
6. Publish a task PR, verify exact final remote head applicable CI, then squash develop only if complete. Report remaining product decisions honestly.
