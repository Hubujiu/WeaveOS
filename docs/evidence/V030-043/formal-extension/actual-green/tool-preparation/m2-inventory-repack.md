# Maven inventory representation repack

Root reviewed original Git blob `913c0fe7150f7dc38bec10c552da50a8f5145fe3` and confirmed all entries are relative Maven artifact paths mapped to lowercase64hex file digests. This change preserves the same 3100 entries and order as JSON two-element arrays [path, digest]. No scanner rule, allowlist, dependency, business code or test changed.

Original object bytes SHA256: `795851b3ad0ae91d1cf3314e0ee90a4bf093f12ffe4a47fa71dd3100e02d2b9d`.
Repacked array bytes SHA256: `00f62d21b22852876ced91ecc4c375367f991864a4bbfaabadd1b00ebbfe50c9`.
New inventory Git blob: `2868f63a00e3007fe8ae119211e8b775de764b60`.

All paths were asserted relative and every digest strict lowercase64hex. dict(pairs) equals the original object; serializing that reconstructed object with json.dumps(indent=2) plus the original trailing newline reproduces every original byte and the exact original SHA256. Original bytes and independently reconstructed bytes are retained privately. Existing run.json capture hashes are unchanged: its tool-preparation/m2-inventory.json entry identifies the precisely reconstructable original object bytes, not this new encoding.

Previous exact source533fc2e14b87292eda5396212ae13ec162d72dc2 scan exited1 with220 generic-api-key findings, all in tool-preparation/m2-inventory.json, lines2–3015, entropy3.618486–3.9386885; report matches are REDACTED. All old per-finding row/column/entropy locations, original report/source.tar/stdout/stderr remain private at /tmp/weaveos-v030-043-gitleaks-ouqcgprg/. Root owns that diagnosis. New full tracked-source snapshot plus updated inventory/this note and newly generated source.tar used the unchanged pinned Gitleaks8.30.1 dir/redact/no-banner/warn parameters: exit0, JSON report empty (0 findings). Original source.tar and stdout/stderr/report retained privately at /tmp/weaveos-v030-043-repack-vpkran9u/. This result-note update is covered by a final same-parameter snapshot scan, with its separate raw result retained there. No commit/push yet.
