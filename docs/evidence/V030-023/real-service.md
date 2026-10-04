# Existing real forms suite — diagnostic only

NOT acceptance: Root rejected the browser isolation override at 21:04 UTC.
The unchanged `apps/web/src/applications/forms/forms.real.spec.ts` produced 12/12 PASS
(4 cases in Chromium, Firefox and WebKit), raw output `green/forms-real.txt`.

The session initially lacked `/tmp/v014-browser-bridge.json`. Provisioned task-labelled,
loopback-only PostgreSQL18.6 (15432), Redis8.2.10 (16379), built the current Go BFF
(18080), ran existing migrations 1–12 and `cmd/acceptance-seed`, logged in through
the real sessions API and created a temporary application through the applications API.
Private credentials/session stayed under `/tmp` mode0600 and are not evidence assets.
The existing test's HTTP route adapter forwarded requests to that actual BFF; it did
not fake responses. This is the existing HTTP service bridge, not a production HTTPS
or least-privilege DB deployment acceptance.

First fixture run lacked NOLOGIN `auth_app`/`auth_backup` roles needed by existing
schema routines, so two Saves failed. PostgreSQL reported `role "auth_app" does not
exist`; adding those roles in the isolated test DB resolved it. Original failed
fixture log retained separately, never counted as product RED. No assertions changed.

Runtime-only config copied original config with an absolute testDir, port4175 and
/tmp outputDir to avoid competing Vite servers; original three browser projects,
timeouts and test source remained intact. Chromium/Firefox/WebKit all ran.
Local outer sandbox required Firefox/WebKit content-sandbox environment adjustments;
installed WebKit libGLESv2 was verified with ldd and its sysroot libraries supplied via
LD_LIBRARY_PATH (host preflight skipped, not tests). Runtime config snapshot included.

After completion, stopped the exact BFF process, removed the two labelled disposable
containers and their anonymous volumes, and deleted private bridge/fixture files.
No repository infra, API, migration, tests, runtime permissions or deployment changed.

Root requires default browser isolation. The local result above is retained only as
diagnostic evidence, not a substitute for standard container/default product CI.
No isolation override is persisted in repository runtime or test configuration.
