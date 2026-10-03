# Inherited R3 save status locator correction

Root authorized only the two save-feedback locators in tests/acceptance/web.spec.ts.
PRD/ADR009 latest readback08:40:01.401Z/08:40:07.467Z; approved R3 v0.2.0
section5.5 requires actual saved configuration and refreshed permissions.
R3 page https://app.notion.com/p/3e92f5a9e64881f4be86c10bc38fff61
updated2026-10-02T08:40:49.691Z. This does not change product semantics or approvals.

Scope coordination b0b5304117ba1ff3078107cd5f0128cd3f026c99 precedes executable edits.
The unchanged original R3 test is retained byte-for-byte. Append diagnostic-hooks.ts
to original-web.spec.ts as diagnostic-web.spec.ts beside diagnostic.config.ts;
select --grep 'R3 real Root UI'. The hooks only delay delivery of the real
background events/search request, then route.continue; no response mocks, sleeps,
production edits or timeout changes. workers1/retries0, default30s/expect5s.
All save POSTs and immediate authenticated GET readbacks reach the real BFF/PG.
Trace/screenshots/videos remain off; synthetic fixture credentials stay private.

Actual RED09:21:07.768–09:21:19.814 UTC at b0b5304: three engines fail the
original line388 strict status locator, with exactly loading span + saved p.
Each template POST201 and same-ID/name GET200 succeeds. red.log.gz is lossless;
red-save-readback.json preserves those safe operation proofs. Production source
and original test are unchanged. The earlier09:19 login timeouts arose from
private dist permissions under cloud umask077 (Nginx500), NOT product RED.
Initial BFF700 mode failed container uid65532; fixed ignored binary755 and
ignored dist directory755/file644. Audit HMAC setup uses existing standardbase64.
No tracked compose/nginx/config/lock file changed. Isolated loopback resources
remain retained for recovery, not production. See stack-safe.json.

GREEN and exact-head checks will be appended after the two-locator correction.
