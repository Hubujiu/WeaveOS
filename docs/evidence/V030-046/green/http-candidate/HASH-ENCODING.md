# Evidence hash encoding

Root reviewed the exact four redacted Gitleaks 8.30.1 findings on 31925112: the two OpenAPI digest properties in generator-run.json and the contracts/openapi/openapi.json entries in both run.json file maps. They are 64-character SHA-256 values, not authentication material. All inventory paths are relative and all values match the SHA-256 format.

For unambiguous machine-readable attribution without key/token-like strings, file hash maps are now ordered [path, digest] pairs; the two generator digests are single-element arrays. Reverse with Object.fromEntries for each map and [0] for each generator digest. No value, path, command, exit status or test identity is removed. artifact_hashes continues to describe the original run artifacts, not this later documentation representation. Original bytes remain in the prior commit 31925112 and private run preservation. This is an explicitly documented representation change, not a claim of unchanged raw bytes.

No scanner rule, allowlist, timeout or test expectation is modified. Full exact-candidate scan must pass before acceptance.
