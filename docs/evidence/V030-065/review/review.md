# V030-065 Root local checkpoint review

2026-10-09 15:44 UTC. Main assistant only; no delegation, remote executor, main merge, deployment, or user-data deletion.

## Source and honest TDD

PRD/ADR/Data bodies authorized by Sentinel_6cc914f4390c8191af06f19f58340059. Relevant source reads and upstream d5c168f tree verified before implementation. Original cleanup RED: real positive publication/execution catalog deletion blocked by legacy FK. Pending/unknown old FK already refused deletion; this was not falsely described as old unsafe cleanup.

Initial migration failed 42501 because FOR KEY SHARE required UPDATE on immutable versions. Exact failed SQL and logs are retained in history-lifetime-green despite the initial intended label; this was an implementation error, not behavior RED. Notion ADR/Data corrected/read back at 15:36 before correction: SECURITY INVOKER, no extra privileges, READ COMMITTED-only write/delete, same app/flow transaction advisory lock, fresh post-wait statements. Six owner RR/Serializable insert/delete tests then genuinely failed with nil before SQL correction, retained in isolation-red. Original complete source and chronology remain recoverable after squash.

## Reviewed implementation

Only new migration28, its exact compatibility registration, tests, and documentation. Existing migrations1..27 and roles unchanged. All original columns, hashes, bytea, actor/operation, receipts and timestamps remain; three stable app FKs replace four catalog lifetime FKs. Full receipts remain full; legacy deployment-only is never upgraded with invented content. BEFORE INSERT verifies original scope; BEFORE DELETE checks active/starting instances, pending/unknown publications, pending execution commands. No product deletion endpoint or DELETE grant.

Advisory collision only serializes unrelated flows; real identity is checked independently. Trigger functions are SECURITY INVOKER, pg_catalog search_path, qualified objects, PUBLIC EXECUTE revoked. Runtime cannot UPDATE versions or DELETE immutable histories. Non-RC writes/delete explicitly25001 to avoid stale snapshot after waiting. Existing RR read paths are unaffected. Real concurrent writer-first and cleaner-first tests observed pg_blocking_pids edges and got55000/23503 after commit respectively; no orphan accepted intent. This does not claim cross-database atomic deletion or complete product deletion concurrency.

Existing publication HTTP implementation required no rewrite: real terminal same operation replay and status remain after full configuration removal, changed fingerprint conflicts, new operation404, missing current session401, no new deployment/audit. Independent original execution event/list/replay also survive full target catalog cleanup; other flow remains.

## Actual verification

- Full ten-package race regression source4726327: 735 top-level +560 subtests PASS, zero FAIL; three pre-existing opt-in appquery capacity probes SKIP, individually recorded. Redis connection-refused logs are intentional failure-path cases, not silently ignored failures.
- Node530 PASS, zero skipped/fail; OpenAPI0 errors/13 existing warnings; go vet/build exit0; task/structure/source diff checks pass.
- Actual private-schema migration rehearsal: original row JSON/bytea unchanged; nonempty Down55000, empty Down restores4FKs then Up works; invalid old receipt binding23503 and whole expansion rolled back. Only schema qualification redirected, no migration logic reimplemented.
- Separate previously stopped history-advisory-green fixture dump under auth_backup and restore: 1 publication+1 full receipt+2 legacy deployments byte-equal; all5 triggers and unchanged permissions checked. This four-row isolated sample is not a production-size or encrypted recovery claim. Full regression used its own fixture; source backup was not a concurrently running cluster.
- Scope/privilege tests reject mismatched view/version UUID and nonexistent legacy version23503; runtime forbidden UPDATE/DELETE42501; original rows unchanged. Active instance and independent pending-command guard55000 verified.

## Remaining bounds

Task remains in_progress. No PR or final exact-head CI yet. V062/V063/V064 actual develop integration must be consumed and rerun before final acceptance. Source-stage local success is not complete forms/approval backend or user main acceptance.

Future product deletion must atomically stop new work, settle unknown accepted results, coordinate actual engine lifecycle, preserve durable original receipts/logs, and reserve deleted flow IDs against resurrection with old receipts (minimal tombstone/delete operation, not retained full BPMN). This migration deliberately does not implement that entry point. Latest-round rework/reopen policy remains unanswered and is not inferred from Notion-body permission.
