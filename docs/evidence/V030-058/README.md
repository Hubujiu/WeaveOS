# V030-058 · Go patch evidence

## Independent requirement and actual RED

The user approved a separate minimal security upgrade after PR65's real source, rollback-source and module gates failed. Official Go 1.27.2 and x/net v0.60.0 fix the October 8 advisories. Existing scanner rules, legacy advisory classification and product behavior remain unchanged.

Root reproduced the original source scans locally with official Go 1.27.1 and the same govulncheck v1.8.0: current source reported nine reachable advisories; preserved rollback source reported six. Both exited 1 with the actual scanner's exit status 3. The new finite configuration guard also failed on 1.27.1 versus independent expected 1.27.2. Tool/download errors are not counted as RED.

## Minimal patch and observed GREEN

- Actual Go builders, acceptance/runtime/CLI/security entry points and six engine interop/generation version guards now use 1.27.2. Official Docker index digest is 5bc7f572bbaa98885a3a1fd9c0aa76b59e3e14e8628bfc316bbfd0c701e4818c. The verified index → amd64 manifest → config chain declares GOLANG_VERSION=1.27.2 and GOTOOLCHAIN=local. This is registry metadata verification, not local Docker execution.
- Current x/net becomes 0.60.0, with only necessary crypto0.57/sys0.48/text0.42/sync0.23 changes. The module language minimum remains Go1.26.0. Old checksum entries retained by `go get` do not change selected module versions.
- Current and rollback reachable scans with Go1.27.2 exit0. Both module scans still exit1 for only existing unfixable GO-2026-5932; actual complete application dependency graphs contain no OpenPGP. This retains the existing classification and is not a zero-advisory claim.
- Both source trees pass Go vet/build, frozen frontend install, typecheck/build and frontend audit. Current flowgraph/workflowrpc/securityreview race run has 53 top-level passes, 99 named pass events including subtests, zero failure/skip. This is not all database/engine testing.
- Configuration guard now passes. Governance/foundation/contracts391 pass. V057's five tests remain in its separate PR, not silently included here.

## Preserved rollback provenance

Original c37226731a6bdbf5c6187aad6cfe1ff9be5daadd remains unchanged. Its module does not require x/net/grpc, so its only source patch is `.go-version` to1.27.2. Local committed candidate e8d499b5506a25d234d1c797131b69ccdb992cfa was actually archived, then scanned and built; native Gitleaks passed. Remote candidate 1bf330852a600103d5a68edcc576a8df6136fcae has the same tree75c0405739d8422b54fb11955d55208d9ec8eac5 and original parent, independently fetched and checked. The connected GitHub app created the commit because local Git has no login; every changed blob/tree matches local bytes.

`evidence/V030-058-rollback-patched` permanently keeps the new source reachable. It must not be cleaned while runtime references it. Only after successful local candidate checks was run.mjs switched to this precise SHA. The archived `.go-version` strict builder check remains, now requiring the patched version. Neither historical references nor production artifacts were replaced; full real container rollback still requires CI.

## Recoverable snapshot and limits

execution-records.json contains123 recoverable UTF-8 records,499157bytes,SHA256 c4868db34d296e766d03e0611f95f4dff0b4b8c403caffc7a52a369de8df8379. manifest.json records every byte count/hash. Records contain original command outputs, scripts, before/after relevant source, tool metadata and actual failures. Re-encode each text field as UTF-8 to recover its bytes and verify hash. It is not an offline dependency bundle: caches, binaries and complete source archives are excluded; use pinned Git sources and recorded versions for replay.

Later exact HEAD scans, PR metadata and CI are separate stages. No local Docker is available; native checks cannot replace the original fixed-image security suite, immutable runtime, restoration or rollback. Complete final-candidate GitHub CI is pending. No main merge, deployment, safety-gate bypass or release approval is implied.
