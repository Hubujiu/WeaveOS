# V030-071 · Root delivery review (local ready)

Local acceptance is complete; final remote CI and develop merge are still pending. This is not a main promotion or deployment declaration. Root writes and tests directly in its own environment. No delegated executor, main merge, or deployment.

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

## Final integrated verification

- PR79 exact b5872f40: all 22 jobs passed; actual product logs confirm hot32/cold7, genuine restore/rollback, two 141-browser passes and expected promotion-blocked. Actual develop squash 4aa36255 was fetched and merged; main6ffba4d stayed unchanged. Both public recovery unions were preserved by semantic comparison.
- Formal hot33 and minimal roles: 13 top-level/30 subtests passed. The exact role suffix is independently pinned, all previous role bytes retain their old hash, extra grants are rejected, and repeated role reset preserves only intended capabilities.
- Real Goose five-case RED→GREEN: empty Down returns exactly32 and re-up33 succeeds; separate configuration and create/update/discard histories each reject55000 with unchanged rows/version. Published migrations1–32 remain byte-identical.
- Formal auth_backup dump/restore preserves32 private configurations and49 minimum receipts plus existing data exactly, restored Goose33 and column-level role boundaries correct. Docker encrypted packaging remains remote CI; no local Docker substitute is claimed.
- Final fixed code39699a64:14 affected packages,969 top-level +917 subtests pass, no failures;3 existing capacity skips are recorded, not hidden. Evidence: final-regression-corrected. The one previous failure was a historical32 rollback test running against populated33; a separate fresh32 database restores the original precondition and retains every original assertion. Product SQL/Go was not changed to make that test pass.
- All560 Node contract/governance/foundation tests pass again, zero skips; full Go vet/build,14-warning valid OpenAPI, exact migration/role pins, altered-role rejection, actual HTTPS response schema checks and task checks pass.
- Source secret scan:133.78MB plus2.83MB delta, no findings; final evidence/doc delta is checked before publication.

## Remaining external gates and product boundary

Publish the complete verified candidate to PR80, mark it reviewable, and verify every applicable CI run/job on its exact final remote SHA. Fix any real failure without relaxing requirements. Only then expected-head squash develop under standing authorization. No main merge, deployment, automatic compatibility override or premature accepted state.

Record/container copy/delete rules remain the unanswered product question; this PR is complete for approved private presets, not a claim that those unapproved features exist. Before entering standby, report what is done and what decision is still needed.
