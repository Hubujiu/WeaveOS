# ADR14.4 private application presets — exact proposal for lead readback

Status: proposal awaiting lead contract readback. Not implemented or available.
Authoritative source: V015 accepted ADR14.4, 2026-10-03 14:47:12 UTC.

Scope is live Session actor + actual app + actual form view, never client owner.
Require real form menu.enter and nonempty data.read at every list/get/write/apply;
no application/directory inheritance, sharing or personnel preset migration.

## Methods and DTOs proposed

Relative `/api/v1/applications/{appId}/forms/{viewId}`:
- GET `/table-presets`: data.items, at most20 private presets, deterministic order.
- POST `/table-presets`: PresetCreate,201 + Location, minimum PresetMutationResult.
- GET `/table-presets/{presetId}`: valid Preset or redacted InvalidPreset.
- PUT `/table-presets/{presetId}`: full PresetUpdate with expectedVersion,200 minimum.
- DELETE `/table-presets/{presetId}`: operationId + expectedVersion query,204 bodyless.
Apply is GET validation followed by existing records/search; no separate apply
mutation, no new query engine, no automatic query refresh/rebase.

Proposed finite closed DTOs:
```typescript
type PresetState = {
  name: string; // TrimSpace,1–100 Unicode, case-sensitive unique per private view
  filter: FilterGroup | null; sort: Sort;
  hiddenColumnIds: (UUID | "createdAt" | "updatedAt")[];
  columnOrder: (UUID | "createdAt" | "updatedAt")[];
  columnWidths: {[columnId:string]: number};
};
type PresetCreate = PresetState & {operationId:UUID; expectedSchemaVersion:Version};
type PresetUpdate = PresetCreate & {expectedVersion:Version};
type Preset = PresetState & {
  id:UUID; appId:UUID; viewId:UUID; version:Version;
  invalid:false; createdAt:string; updatedAt:string;
};
type InvalidPreset = {id:UUID; name:string; version:Version; invalid:true;
  reason:"FIELD_UNAVAILABLE"|"PERMISSION_CHANGED"|"DEFINITION_CHANGED"};
type PresetMutationResult = {operationId:UUID; id:UUID; version:Version};
```

Limits preserve existing independent20 slots, raw64KiB/canonical32KiB;
the application's exact filter compiler remains3 levels/20 leaves, all/own
whole-visible-row coverage; sort remains the existing singular Sort.
Every field ID in filter/sort/hidden/order/width must be live and readable in
the current effective scope; system IDs are the frozen query system columns.
Invalid GET/list omits all criteria, operands, field IDs, widths/order and
timestamps; original definition remains stored. A user with current menu/read
may explicitly PUT a valid replacement or DELETE using current preset CAS.

## Lead decisions required before implementation

1. Approve exact route/PUT, minimum write receipt, DTO member names and safe
   invalid reasons above. In particular column IDs: should all five readonly
   system columns be permitted in display settings, while only createdAt /
   updatedAt remain filter/sort columns?
2. Confirm width numeric bounds/unit from the shared Table contract (proposal:
   positive safe integer CSS px, no new arbitrary clamp), order is a duplicate
   free partial order; omitted columns retain shared Table's established order.
3. Confirm that a harmless schema/view change does not alone invalidate a saved
   preset: validate all current fields/kinds/coverage; deleted/retyped criterion
   or missing coverage invalidates the whole stored definition. expectedSchema
   applies to new save CAS; stored field-kind signatures detect semantic changes.
4. Approve APPLICATION_PRESET_NAME_CONFLICT / LIMIT_REACHED / CONFLICT,409;
   use existing RESOURCE_INVALID,FORBIDDEN,NOT_FOUND,SCHEMA_CONFLICT,
   OPERATION_CONFLICT,OPERATION_UNCONFIRMED and common validation/unavailable.

## Ownership / transaction allocation

New `services/bff/internal/apppresets/`, owned shared HTTP helper/routes under
apprecordhttp, additive hot13 candidate (number recheck before reservation),
hot roles minimum named-table grants and finite operation kinds/results. No
cold value storage. Register exact paths before edit after lead freeze.
Lock order: configured limits → global query revisions / source guards → live
actor → actual app/policy / operation replay → claim → table gate → actor/view
preset serialization → preset row CAS → metadata audit/minimum result → one
COMMIT. No record data write/fence/Flowable call; confirmed replay is actor/key
minimum recovery before stale permissions/CAS. No automatic mutation retry after
COMMIT unknown. Read validation is one live readonly RR; apply uses unchanged
records query token verification.

Independent real PG/Redis/HTTP cases:20/21 slots, Unicode/name collision, private
actor/app/view isolation, cross-view same table, real menu/read revocation,
field/type/schema invalid redaction (no operand/field-ID leaks), explicit repair
and deletion while invalid, concurrent CAS, idempotency/unknown COMMIT/rollback,
original queryVersion after preset apply, hidden fields not shrinking P,
personnel original IDs/rows unchanged, restricted roles and backup/restore.
