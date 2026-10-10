# Implementation preparation, not completion

Sources: approved AP-FR-08 and V073 PRD/ADR, read 2026-10-10 16:05–16:14 UTC. The exact user question/answer was read again; no repeated approval is pending.

## Observed invariants
- `operations.app_id` retains application identity. Record history, workflow instances/evidence, drafts and presets retain view identity using RESTRICT foreign keys.
- V072 recoverably deleted rows still occupy their original typed table and must count as nonempty. They cannot become a table-delete purge path.
- Current application creation creates its own registry/menu, no default custom permission group. Custom groups can be disabled; membership and grants have explicit replace APIs.
- Original B5 excludes choosing permission-group deletion consequences. Do not silently add a group-delete operation under the approved container deletion task.
- Existing template export/import is structural-only, uses current manager/create authorization, explicit external-reference mappings, new internal UUIDs and disabled workflows. Application copy can reuse this core, but copying partial subtrees into an existing application needs its own complete contract.

## Consumers to classify before implementation
1. Current app/menu/structure visibility and manager/direct-resource access.
2. Record query/count/fingerprint/get, ordinary edits, drafts, private presets.
3. Definition load/save, other-view layout dependencies and directory moves/association.
4. Workflow configuration, publication, reservations, current task operations, unknown fences and historical evidence reads.
5. Template export/preflight/import and future copying.
6. SQL finite capabilities and metadata insert/update/delete invariants; preserve old functions and restrict private entry points.
7. Existing actor-operation replay before current resource eligibility; no new writes to deleted scopes.
8. Minimum role changes, full backup/restore, guarded Down and compatibility hashes.

This inventory is not proof that every call site is safe. Exact nonempty definitions, deletion-marker representation, HTTP/CAS shape and migration are still to be frozen in Notion before behavior tests/implementation. V073 has no runtime changes or test success claim.
