# PR26 / controlled focus repair integration

The approved B5a.18 repair is integrated after the reviewed backend and shared
backup fixture at 684e599. Late exit completion now shares the same single-use
close authority as Base UI finalFocus; a later deliberate focus/navigation,
reopen or unmount revokes it. Normal close and removal-induced body focus keep
their restoration behavior. No layout, CSS, animation curve or shared helper
changed. Exact implementation remains the parent's reviewed producer bytes.

Root scope registration 103522d precedes four sequential `cherry-pick -x`
commits. verification.json records original-to-integrated mapping and 41
byte-identical source/evidence files. Both the entire primary RED test source
and inherited tests remain byte-identical prefixes of the final test. The
producer's original RED, later explicitly labelled replay and disclosed setup/
type fixes remain unchanged in ../filter-focus/. No new RED chronology is
claimed for this integration check.

Integrated cloud checks use Node24.14.0, pnpm10.28.2, Playwright1.63.0 and saved
Chromium/Firefox/WebKit engines, one worker and zero retries. Q36 passes159,
fails0 and retains9 existing capability skips; manager passes72 with no skips.
The deterministic target passes in all three engines as part of Q36. Governance/
foundation222, contracts33, pure-filter4, typecheck/build, repo and actual PR26
scope pass. Build reports the existing large-chunk warning. Raw results and
UTC command times are retained in verification.json and lossless *.log.gz.

The cloud uses saved browser files and private host-library sysroot. Host
requirements validation is disabled only for that saved private-lib setup;
no browser project/test is skipped by this environment setting. Firefox's saved
content-sandbox setting is retained. Frozen-lockfile install initially failed
because default pnpm-home was unavailable; private workspace PNPM_HOME/store
recovered it without repository configuration or dependency edits. One wrong
contract test glob ran zero tests; that result is invalid and not counted.
The corrected actual contracts/*.test.mjs command executes and passes33.

Pinned unmodified Gitleaks v8.30.1 tracked-source replay at tested integration
HEAD reports zero findings. The final documentation HEAD is scanned separately
before normal push; remote CI terminal status is checked on that exact pushed
HEAD. This evidence describes local verification and does not claim future CI.

The additional close authority uses O(1) per-instance state and a fixed set of
three document listeners; no list/data traversal, HTTP or database call is
added. Active-element containment uses the browser's ordinary DOM ancestor
check. Existing animation enumeration and business logic are unchanged. No
capacity, timing benchmark or production-performance claim is made.

The accepted Library backend v1 and focus repair v0 archives are preserved;
no overwrite or new transfer occurred during integration. PR21, main and the
fixed integration anchor were independently rechecked unchanged. Backend Go,
OpenAPI/errors, migrations, roles, backup fixture, dependencies and workflow
bytes equal684e599 throughout this frontend integration. New-app UI, full v0.3
acceptance, dynamic DDL/Flowable and production operations remain outside scope.
The fresh local Go/PG/Redis suite is not rerun for unchanged source; exact-head
remote CI performs applicable backend/runtime/product regression.

Task remains in_progress while exact-head CI is pending; the draft PR remains
open. No main merge, deploy, production operation, permission expansion or
whole-v0.3 acceptance. Active task worktrees and private recovery resources are
retained because the PR is unmerged. Next: final-head scan and normal push of
only task/V030-011-applications, then monitor all applicable remote CI terminal.
