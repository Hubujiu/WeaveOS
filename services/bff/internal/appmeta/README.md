# B1 metadata invariants

This package implements the limited B1 PLAN in the v0.3 PRD and ADR009. It is
internal Go code, with no registered HTTP endpoint or storage/runtime adapter.

`Catalog` keeps application-center grouping separate from the per-application
directory tree. `Structure` keeps logical table definitions separate from views;
multiple views can reference the same table. Names and array positions do not
participate in identity. IDs are opaque because their generation and portable
encoding are still root-owned decisions. The representation does not impose a
product rule for root directories, membership cardinality or name uniqueness.

`ValidateStructure` validates all ownership and reference constraints and uses
iterative traversal for self, multi-node and disconnected cycles. `Reader` must
supply one coherent application snapshot; `ReadStructure` checks the actual
returned owner against the request and validates the snapshot before returning
it. Authorization must happen in the owner integration. No write/CAS/storage
atomicity implementation is implied by the reader contract.

`StructureTemplate` is a metadata projection, not the full manifest format.
Unrecognized sections (including records, business attachments, secrets and
credentials) are rejected. Opaque field/layout/workflow/role sections are also
rejected until root integrates their reviewed component contracts; labeling a
payload as an allowed section does not authorize arbitrary nested JSON. The
future parser must retain all sections and cannot silently discard unknown data.
This package does not parse archives or claim to detect arbitrary secrets hidden
in unrestricted display strings.

`PreviewStructureImport` produces a non-executable snapshot: workflow source
enabled flags are discarded, all workflows remain disabled, and member/
department/permission references remain unbound. It performs no ID remapping,
binding, grants, persistence or workflow execution. Accessors return independent
copies so callers cannot activate workflows by mutating a returned slice.

Root integration is needed for actual metadata write/schema/CAS/authorization,
central entry registration and transactional recheck, application audit, full
template manifest/component schemas and binding/remapping/atomic import. Field
schemas/DDL are B2-owned; executable workflow schemas/runtime are B3-owned.

Run with the saved cloud Go toolchain:

```sh
go test -race -count=1 ./internal/appmeta
go vet -p 1 ./...
```

From `services/bff`, these tests cover only the package contract. See
`docs/evidence/V030-002/` for authentic RED sources/logs, GREEN/race/vet/build
results, synthetic benchmark observations and the remaining integration gates.
