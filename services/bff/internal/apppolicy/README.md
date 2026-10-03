# B4b internal policy evaluator

This is the bounded pure Go core authorized by the PRD/ADR009 B4b PLAN dated
2026-10-03. It has no HTTP handler, SQL, Session lookup, permissions cache, source
hook, mutation, or side effect. It is not a complete permission system.

## Inputs and trust

The trusted server caller must authenticate the actor and supply one coherent
policy snapshot, the persisted fixed application owner, effective group/member
facts, and the existing resource's actual application/kind/ID. TrustedActor is a
contract, not cryptographic proof; copying a client Bootstrap flag into it would
violate the contract. No request decoder is provided.

For data checks the caller resolves the row through the actual target form/view,
verifies that association, and injects its application and immutable CreatedBy.
LastEditor is informational and never supplies own scope. The evaluator checks
application/resource existence and same-application row facts; it cannot prove
that a caller's asserted facts came from storage. Fields must already be checked
against the target schema by the caller. IDs/kinds are opaque exact identifiers;
names, hierarchy and resource kind do not infer authorization or inheritance.

## Internal contract

- CanCreate: a nonblank authenticated actor with trusted Bootstrap or CreateApp.
  This does not grant access to any existing application. Ownership is not a
  global create capability; losing create permission does not remove ownership.
- Allows: one menu entry or one record/action/field cell. The finite primitives
  are menu.enter, data.read and data.edit. Unsupported actions fail closed even
  for privileged actors because they are outside this package's action domain.
  This is not a declaration that Bootstrap lacks an approved future capability.
- AllowedFields: project only requested data fields allowed for that exact row
  and action, unique and in request order. Results do not alias the input.

After checking actual context, trusted Bootstrap and the application's fixed
owner have full authorization for all supported primitives. Ordinary requests
use enabled same-application groups whose member list contains the actor. Each
grant binds exact resource, action, row predicate and field mask. Complete
grants are ORed; masks are unioned only after resource/action/row checks. Empty
groups/grants do not create a deny rule. Empty data masks allow no fields. There
are no wildcard fields, subordinate scopes, explicit denies or parent grants.

Menu grants use all scope and an empty field mask, and do not require a row.
Menu entry and data checks are independent: the product caller must enforce
both relevant gates. An own record predicate cannot supply menu entry.

All row scopes are all or own; own compares CreatedBy to the actor. DataRead
and DataEdit are separate actions with their own masks. For a multi-field write,
the caller must check every changed field and reject the whole write if any
field is unauthorized, then perform business validation and an atomic write.
This module never performs or partially applies a write.

Full authorization does not establish schema/type validity, dependency safety,
task assignment, node field whitelist, recordVersion, workflow state or approval
eligibility. No approval/reversal/completion operation is implemented. A data.edit
decision cannot be used as proof of those independent business conditions.

## Cost and execution properties

Let G be groups inspected, M their member entries inspected, H grants inspected,
S matching field-mask entries, and Q requested fields. String comparisons/hash
costs also depend on opaque ID lengths.

- CanCreate: constant state; time includes scanning the actor ID for nonblank.
- Allows: worst O(G+M+H+S), constant auxiliary memory. Privileged checks return
  after context validation, without scanning groups. Early matches may return.
- AllowedFields: expected O(G+M+H+S+Q), O(S+Q) auxiliary/result memory using Go
  maps. Only matching complete tuples contribute fields. Privileged projection
  is expected O(Q) time and space. Map complexity is expected, not worst-case.

Functions do not modify input or cache results. Callers must keep the snapshot
immutable for the duration of evaluation; concurrent caller mutation is outside
the contract. No benchmark or production performance claim is made.

## Validation and remaining integration

Independent tests cover create-only/owner/Bootstrap, bad actual contexts, exact
resource namespaces, effective memberships, all/own, read/edit separation,
cross-group Cartesian-product prevention, menu/data separation, unsupported
actions, stable field projection and replacing snapshots. RED and GREEN use the
identical test-file hash. Evidence is in docs/evidence/V030-010.

NOT RUN / outside scope: real HTTP/Session spoofing, permission persistence and
revocation CAS/concurrency, role/group CRUD, delegated grant limits, owner
transfer, resource inheritance, SQL row filtering, field serialization, schema
validation and Flowable/task qualification. Future action registries, source
hooks and host adapters require root's separate plan. No production account or
database role was changed; no existing PR was updated, pushed or merged.
