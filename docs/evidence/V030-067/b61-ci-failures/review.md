# Exact-head CI failure inspection

2026-10-09 20:54 UTC. Candidate b61de621a8d8584d4b652e27e214c06526ed16aa, PR74, CI run37978254922. Not ready.

- workflow-formal-runtime job113982490981: TestRootFormalRuntimeReturnThenWithdrawRecovery fails in startup, root_main_process_test.go readiness sees BFF child done. Current helper does not print child wait error or exit status; child stdout/stderr is empty. The other six top-level/subcase formal scenarios pass. Root cause not yet established; no assertion/source changed.
- browser job113982490885: Q36 B2 explicit member draft case exceeds unchanged15s timeout in page.goto('/app/admin'), waiting for load. Saved Chromium trace records external fonts.googleapis.com stylesheet taking13201.488ms. App tab is already rendered; navigation resolves only during teardown and business assertions then cannot complete. Not evidence of a deletion-RPC regression. No test timeout/skip/assertion changed. Need deterministic isolation of unrelated remote stylesheet or verified same-head rerun, not fabricated green.
- Workflow engine, deployment RPC including actual new deletion interop, execution RPC, Go, governance, recovery/backup/schema and product workflows passed. Overall CI failed and merge is blocked.
- Formal artifact11639574408 SHA256 bb0147921df56bcd84b29b3dfd2b71a9ec53de099552298ad9368a30b681ae87; browser artifact11640810938 SHA256 768be8a14fa6f0cf8149902867f2687da5e5e7ff72b943ae78c3eb7c7091de3c. Downloaded through GitHub then supported local file materialization, actual ZIP checksums verified. Formal ZIP and precise failing browser trace retained here.
- A local exact-source repeated formal process reproduction is running with isolated PG18.6/Redis8.2.10/Java17 and real runtime, not Docker. Outcome pending. No claim of reproducing container memory/isolation.

GitHub evidence upload was rejected by authorization review as conflicting with no-Codex-Cloud restriction. Publishing paused; user clarification requested20:54. Local investigation continues. No alternative push route used.
