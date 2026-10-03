# PR26 isolated CI setup recovery

Initial reviewed backend source is 9668a4e55c49431d60adf45f2a4a93047db3f6ba.
It is retained as an ancestor; product behavior and frozen inputs are unchanged.
The stack base is the exact four-input integration anchor 08e8a97, without
source changes, so the PR diff is only the owned V030-011 package.

Initial governance job 111135012194 in run 37099125289 failed with
`PR/task identity mismatch`: pr:null was the historical local-only metadata.
The real PR26 number was written by documentation commit 2498ec4; its
actual-PR scope check and 222 governance/foundation tests pass. The next remote
governance job 111135360953 passes. No task checker/gate was modified.

Remote Go job 111135361014 in run 37099228736 actually failed because the fresh
migrated cluster had no auth_app role. Applications tests ran before the audit
package that previously installed reviewed runtime roles. This is an isolated
fixture/environment defect, not a missing-product-behavior RED. Existing local
GREEN had correctly used the reviewed role policy, but externally initialized it.
The actual remote log says:
`ERROR: role "auth_app" does not exist (SQLSTATE 22023)`.

Repair changes only the applications integration fixture: the isolated migration
owner loads the exact repository infra/runtime/roles.sql before creating any
restricted pool. No permissive test-only grants, auth bypass, assertion change,
skip, production role change or product startup role initializer was added.
The reviewed policy remains byte-identical; test binaries alone include this
setup. This does not modify CI/root dependencies or shared workflow files.

A separate fresh PG18.6 cluster, cold3 then hot6, independently reports no
`auth_app` before the fixture. No role policy is applied by the external runner.
The existing application race tests then exercise their real restricted role.
The setup and source hashes are preserved with the fresh run; original failed
fixture bytes are recovered from the exact immutable remote HEAD in the archive.
That archive was created after diagnosis, not falsely labeled as created before
implementation. Target COMMIT/query-count RED test files remain unchanged.

Sources were re-fetched on publication: PRD 04:03:08.028Z / ADR009 04:03:15.231Z.
Frozen B5a.1–9 match; later source sections contain bounded root acceptance.
05:03:38 UTC user authorized push/draft PR; no merge/deploy/production authorization.
No astra or local-machine work was used. Existing reviewed Library artifact is
retained, not overwritten by publication metadata or test fixture recovery.
