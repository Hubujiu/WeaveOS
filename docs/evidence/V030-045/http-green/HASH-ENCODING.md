# Reversible evidence digest representation

Root reproduced the exact PR55 archive with fixed Docker Gitleaks v8.30.1. Six generic-api-key findings were file SHA-256 metadata: two generator digest properties and four OpenAPI inventory entries. All original inventory paths are relative and digest values are 64 lowercase hexadecimal characters. No authentication material is involved.

Only representation changes: the two generator digests are single-element arrays; hashes_before, hashes_after and artifact_hashes use ordered [path, digest] pairs. Reverse with [0] and Object.fromEntries. Every digest, path, result and test identity is retained. Artifact digests describe original run bytes, not these later reformatted documentation files. Original files remain in commit3d793c6f1bb88b8889b483a1fead81bce13e26be and private preserved evidence. This is not a claim of byte-identical reformatted evidence.

No scanner rule, allowlist, business source or test assertion changes. Full archive scanning of this exact candidate is required before final acceptance.
