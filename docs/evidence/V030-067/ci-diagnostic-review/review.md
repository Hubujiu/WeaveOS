# Root review: preserve failed formal-process evidence

2026-10-09 21:04 UTC. Main assistant only. No production BFF/Java/SQL/protocol code changes in this repair.

## Verified failures

Exact remote b61de621 CI37978254922 still failed. Browser trace shows original page.goto waiting for Google stylesheet13201.488ms and exhausting unchanged15s test timeout. No browser assertion, timeout or selection changed. Installed local Chromium could not launch due denied socket operation; optional navigation experiment is explicitly NOT RUN, not a red/green claim.

Formal BFF startup exited once on GitHub. Original helper swallowed failure evidence: its owner's cleanup was registered before child's first t.TempDir; Go1.27.2 removes that directory first. Actual helper test-only overlay reproduces missing original log. This explains empty diagnostic text, not why original BFF exited.

## Genuine repair sequence

1. Existing helper called by a real /bin/sh child that prints a fixed diagnostic plus synthetic token/password and exits23. Isolated recursive test process intentionally fails, outer test demands preserved redacted child text and actual exit status; original helper failed both assertions. Full original test source/output and times retained. Test renamed only from FormalChild to FormalRuntimeChild so existing formal selection includes it; assertions unchanged.
2. Independent frozen formal gate test added this one identity while preserving all old6 top-level/2 subcases: valid expanded report rejected and missing-diagnostic report wrongly accepted, observed1failure/1error. No old identities, skip bans or report completeness checks removed.
3. Minimal helper fix registers child stop-and-log after allocating its temporary directory, so read happens before deletion. Owner's duplicate now-too-late log calls removed; owner stop stays idempotent. Failed child ProcessState supplies exit/signal status; missing reads produce explicit bounded diagnostic, existing credential redaction retained. No readiness, shutdown, query, HTTP or test budget increased.
4. Same real failure test passes under Go race. Seven independent parser tests pass after frozen gate adds new identity; total now9 including old cases.
5. Fresh isolated actual PG18.6/Redis8.2.10/formal Java17 and actual compiled BFF regression:7top-level+2subcases PASS, nofail/skip, exact formal report gate accepts9. This is local process acceptance, not Docker CI. Before repair original b61 source also passed5 complete repetitions30top+10sub; original GitHub transient exit remains unconfirmed.
6. Full Node530 PASS, nofail/skip; task/structure/diff checks pass. Original private CI formal artifact47.27KB scanned by pinned Gitleaks8.30.1 with no leaks before retaining. Final source scan follows commit.

## Review boundaries

Independent owned cleanup now stops children in reverse creation order before earlier fixture cleanup; all children are already stopped before proxy/connection cleanup, with wait channel synchronization. Restarts and forced-kill recovery are covered by actual process suite. Only test-only files and strict report baseline changed. New diagnostic tests cannot fake a real product process acceptance result: old formal cases and strict identities remain mandatory.

Remote writing paused after review interpreted connector publishing as prohibited Codex Cloud; user clarification20:54 remains unanswered. No alternative publishing route used. This candidate cannot merge until published and exact GitHub CI succeeds. Browser failure still needs remote verification/appropriate isolated fixture work. Full app delete worker/HTTP remains unfinished; latest-round policy unanswered. main/deploy/user data untouched.
