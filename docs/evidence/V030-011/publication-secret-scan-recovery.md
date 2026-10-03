# PR26 source-digest metadata scan recovery

At bfd5c9f, product job 111137439223 in run 37099975476 passed migrations,
HTTPS and real product acceptance, then failed the tracked-source Gitleaks gate.
The security stage passed 25/26 tests; later restore/image/package steps were
skipped. This is not a claim that the whole product workflow passed.

The exact pinned Gitleaks 8.30.1 image, git archive HEAD plus extracted source,
and existing redaction settings reproduce exit 1. All 89 findings are explicit
SHA-256 source-file digest mappings in 45 owned evidence JSON files, under the
same generic-api-key rule. Five of the six distinct path/digest pairs match Git
history; the remaining pair matches the retained corrected RED source archive.
The classification report records file/line/rule and byte-source proof, without
candidate credentials or private scanner output. No genuine secret was found in
these 89 findings. This statement concerns this scan, not repository history.

Only the display metadata is converted from a filename-to-digest mapping to
explicit {path, sha256} records. Every digest and all other semantic fields are
unchanged. Original JSON bytes are preserved exactly in the adjacent originals
tar.gz; extraction and semantic round-trip checks pass for every file. Original
RED/GREEN streams, tests, source archives, commit history and reviewed Library
v1 remain unchanged. Archived originals are historical raw records; the JSON
files are the readable representation and their conversion is not a new test run.

No scanner rule, allowlist, gate, ignore path, assertion, skip, workflow, runtime
code, contract, migration, role policy or dependency is changed. No source-digest
consumer exists in scripts/tests; the representation is documented here for
reviewers. This is evidence formatting, TDD N/A; the actual unchanged security
scan is the verification oracle. Remote final-head CI remains pending.

Actual metadata HEAD 6b7063c677bee6dbc6e2285591e85aa1e026d321 now scans with
the same pinned image: exit 0, zero findings. The exact unmodified existing
tracked-source security test also passes, without diagnostic extra flags.
All 555 digest pairs and raw originals pass a fresh independent reconstruction
check; governance/foundation 222, real-PR task scope and structure checks pass.
The final evidence-only follow-up will be rescanned before normal push.
