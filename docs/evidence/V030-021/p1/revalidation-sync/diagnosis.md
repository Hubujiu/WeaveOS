# PR33 read-only revocation diagnosis

Frozen head: 97589d3bb36bf229fd389a9430d66ef3cd0a3a93. Production unchanged since f518200. No forms source/test/harness changes vs db475e0.

CI product attempt 1 failed forms.component.spec.ts:1092: dialog count expected 0, got 1. Original log: /tmp/weaveos-pr33-product-failed.log. No original CI failure trace is available; artifact download failed. Consequently the actual CI timing is not conclusively reconstructed.

Local original test: 10/10 plus 50/50 pass. Observational original-sequence probe: 60/60 had B commit, A return request with 403, zero dialogs.

Controlled merged A->B->A state setters: no B commit, no return request, no 403, original dialog remains after 5 seconds. This proves a possible batching failure mechanism, not that CI took that path.

Controlled committed intermediate B: B commit, return A remount with verified=false and cached dialog in state, A GET with expected actor 181 returns 403, permission error appears while DOM dialog count remains zero. Actor is unchanged throughout; this exercises app revalidation, not actor replacement.

Source: harness.tsx:13,16 exposes React setState directly, with no commit acknowledgement. ApplicationStructurePanel.tsx:23-25 keys scope by actor/app; :40 starts verified=false; :90-111 refreshes on mount/scope, catches 403 and keeps verified false; :200 excludes dialog markup until verified. The setter being returned from page.evaluate does not itself establish a React commit.

Root-owned candidate fix: forms.component.spec.ts:1087-1091 synchronize on the intermediate application commit or its structure request (registered before switching), then on the original app's 403 response (registered before switching back). Preserve original dialog/private-name/permission-alert assertions. Adjacent test at :1097 repeats the same unacknowledged two-switch pattern. No production change is justified by observed evidence. Do not broaden scope or edit Root tests without Root instruction.

Probe code /tmp/weaveos-revocation-probe.mjs; results /tmp/weaveos-revocation-probe/results.json; traces committed-0.zip and coalesced-0.zip in that directory. Synthetic fixture data only. Observation hook can affect timing. Slow CPU original-order probe runs separately under /tmp/weaveos-revocation-slow.

Slow CPU (20x throttling) original-order probe completed: 100/100 B commits, 100/100 return A 403 responses, zero remaining dialogs. No natural failure reproduced. Results: /tmp/weaveos-revocation-slow/results.json.
