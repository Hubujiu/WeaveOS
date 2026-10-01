# Approved Q36 migration compatibility registration

Root explicitly coordinated `infra/server/deploy/compatibility.json` after identifying the omitted approved 00003/00004 entries at PR head `7afc8a5`. The original strict policy rejection (`Migration not approved`), pre-fix JSON and independently recomputed canonical LF migration digests were saved in RED commit `0abd78b`. The additional registration driver initially rejected the missing exact approvals; its raw RED is retained separately.

The minimal production change adds exactly:

| Migration | Fixed canonical LF SHA256 |
| --- | --- |
| migrations/00003_query_drafts.sql | 9b1f546d158931d09245070a4d5b6b46dd64b128b008363ae7a44f74bae8860a |
| migrations/00004_query_revision_writers.sql | 85faf7406d0e82ce7ca14aaf10276f80ef5a88a5c957170f111a261e910c925d |

Source text now includes ADR-007/Q36 and the requirement to upgrade the reviewed administrator-installed fixed receiver/role policy together before migration promotion. Four published hot/cold migration pins, both compose/nginx pins and backwardCompatible semantics are unchanged. No wildcard, unknown migration approval, edited migration, permission expansion or production installation was introduced.

Backward compatibility is supported by the existing real tests, not merely the flag: a fresh isolated cold-before-hot expansion from 00001 through current Q36 migrations retains existing auth/personnel data, accepts old auth-only writes before and after upgrade, preserves read-only backup, and checks the approved draft/revision/lock permissions. The separate transactional migration failure and application rollback retain existing rows/Goose version. Both files were re-run in new containers after registration: **4 passed, 0 skipped**. The approved migrations remain expansions; published migration bytes were not changed and no destructive Down was applied.

`verify-registration.mjs` independently checks the exact six-entry approval set, all canonical current migration/config bytes, unchanged published pins and installed receiver roles pin. It executes the unchanged production validateCompatibility/validateEnvelope/receiveBundle code and rejects unknown migration, tampered Q36 digest/content, rewritten published migration and false compatibility declaration. Existing deployment/personnel governance: **18 passed**; complete governance/foundation: **132 passed**. Structure/task/check-release checks passed; check-release is structural recorded evidence completeness, not current product/user acceptance.

## Actual immutable OCI packaging verification and its source boundary

There was no current Q36 accepted OCI export in this Windows workspace. The newer product runs stopped before their tested-oci upload. We downloaded the original **tested-oci** artifact from successful prior PR [run 36827885780](https://github.com/Hubujiu/WeaveOS/actions/runs/36827885780), artifact id 11146578081. Its source is the workflow merge commit `895acd2e5ba92d4acf552e80b8c266e25db6fe1a`; the run's PR head was 09707d9. These identities were preserved, not relabeled as the current Q36 head.

The real unmodified `infra/server/deploy/package.mjs` imported those exact OCI files, verified archive/manifest/config/layer/source identities, used GITHUB_SHA equal to their accepted source, saved both existing images and generated the release envelope. There was no image rebuild. First, an isolated fixture directory with the archived pre-fix compatibility mapping reproduced the actual CLI error `Migration not approved` after loading the accepted OCI; **this full-CLI RED is explicitly post-fix REPLAY**, not the initial RED. It did not overwrite checkout/config or another worktree. With the current reviewed mapping the same CLI exited 0. The resulting manifest preserves original OCI digests/checksums and was verified against accepted BUILD.json; the complete release.bin was read through receiveBundle, and both Docker archive byte streams passed checksum/size validation. Unknown metadata and tampered 00004 contents were rejected. Input OCI archive checksums remained unchanged.

This verifies the actual packing/import/binary transport path using historical accepted OCI input. **It does not claim those historical images implement Q36 or that the current Q36 same-HEAD OCI product chain has passed.** The new final CI performs that complete current-source chain; root owns its terminal result and acceptance. Public evidence contains metadata/logs only; OCI files, release.bin and temporary fixture/received image archives remain ignored under `.work/q36-compatibility`.

```powershell
# Exact historical accepted OCI input; no identity substitution or rebuild
gh run download 36827885780 --repo Hubujiu/WeaveOS --name tested-oci-895acd2e5ba92d4acf552e80b8c266e25db6fe1a --dir .work/q36-compatibility/accepted-oci-36827885780
$env:GITHUB_SHA='895acd2e5ba92d4acf552e80b8c266e25db6fe1a'
$env:GITHUB_RUN_ID='36827885780'
$env:GITHUB_RUN_ATTEMPT='1'
node infra/server/deploy/package.mjs .work/q36-compatibility/accepted-oci-36827885780/BUILD.json .work/q36-compatibility/pack-green
node docs/evidence/V010-020/q36-b2/compatibility/verify-registration.mjs .work/q36-compatibility/pack-green/manifest.json .work/q36-compatibility/accepted-oci-36827885780/BUILD.json .work/q36-compatibility/receive-green
```

## Remaining chain registration review

All current .sql files in hot/cold migration directories now have exact approvals; published pins and compose/nginx hashes match. The fixed receiver role digest matches current roles.sql canonical LF bytes; the source/BUILD/OCI labels, manifest/config/layers and unchanged archive identity checks passed through actual import. Packaging uses the exact exported accepted pair without a build call. Historical V010-019 digest records remain historical evidence. check-release matrix had no structural missing entries; its workflow gate runs only when verify_release is requested. No other same-scope omitted registration was found and no product decision or gate was changed. Final current-source OCI product run remains the root's final verification.
