# V030-046 integrated execution evidence

Actual executed backend source was the frozen 0ea9ddfa + 3d793c6f integration resolved by Root c2fe7943. The final parent candidate is 954de6f3142cc2229becd159e06f6cc3d98b0c5b. Tests were not replayed for this evidence copy.

- Target service: 57 top-level and 10 subtests PASS.
- Target HTTP: 12 top-level and 13 subtests PASS.
- Default four packages: 445 top-level and 185 subtests PASS; no FAIL/SKIP.
- Existing V046 isolated hot database upgraded 21 to 22 to 23. Only 023 was subsequently reversed and reapplied: version 23 -> 22 -> 23, scoped index count 1 -> 0 -> 1, instance count remained 23. Original V045 database was not connected by that run.
- Migration files 001-022 retain exact hashes against lifecycle parent 3d793c6f, recorded in checks/invariants.json and verified again against the final candidate.
- Full archive of exact remote parent 954de6f3 scanned with pinned official Gitleaks 8.30.1: exit 0, zero findings. Archive hash and scanner arguments are in scanner/run.sanitized.json.

Backend raw stdout, stderr, exit files and source snapshots are byte-identical copies; raw-copy-hashes.json records ordered [path,digest] pairs. The original private run.json is retained privately, with its hash recorded in the explicitly sanitized repository version. Sanitized metadata is not claimed byte-identical: fixture metadata/docker argv is omitted and local tool/worktree paths are normalized. No environment dumps, credentials, private keys, database state, tool caches or scanner source archive are included.

Production/test/SQL/dependency hashes stayed unchanged during execution. Entire source_tree_unchanged was false because exactly three Root-authorized 1df390a9 document hash representations were updated during execution; backend/document-representation-update.json records the difference. Q36 evidence was later copied without changing tested production/test/DDL bytes. Inherited formal-runtime evidence is not a fresh seven-case runtime run of this combined candidate.

The full whitespace check remains exit 2 on six pre-existing raw-log lines. Root approved byte preservation and three exact-path exclusions; the scoped source check is exit 0. Original diagnostics and exception provenance are retained in checks/. No task status or branch/PR reference is changed by this evidence candidate.

The raw full whitespace-check stdout is stored as deterministic gzip. Decompression was verified byte-identical to the original; lossless-compression.json records original and stored hashes. This avoids adding any new whitespace exclusion.
