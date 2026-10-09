# Exact ef343 browser failure diagnosis

CI 37922197165 job 113793005894, Firefox engine Escape ordinary-close case. 119 passed, 1 failed; subsequent saved-filter and smoke steps NOT RUN. No test retries/skip/timeout change.

Artifact 11612788323 downloaded 2026-10-09 11:25 UTC, 4,276,852 bytes; SHA256 9e7ea3b545ce774606a721ee9cf84064cfb624b44d91296e5252dd40e67f11d8 verified. Original archive retained in current task environment. This archive preserves original raw Playwright event and network streams, not regenerated evidence.

Trace: test starts 252514.507ms; goto(load) begins 252751.841 and ends 268183.172 (15,431.331ms). Test timeout occurs before goto finishes, After Hooks starts 267520.024. Button click/visibility/enabled calls occur afterward during teardown. The observed disabled button is not evidence of the focus-restoration behavior failing.

Network confirms Google Fonts CSS took 13,238.773ms (wait 1,798.363 + receive 11,422.976), versus local assets at most 130.458ms. Screenshot inspected: panel already absent during teardown. No manual-options request or backend execution is involved in this mocked component case. This is a verified external-font loading delay, not a rationale to weaken focus assertions.

A new exact-head CI is already required by the independently found OpenAPI correction. Verify that full run rather than blindly rerunning successful backend jobs here. If the external dependency recurs, separately scope deterministic browser asset isolation with original behavior/assertions preserved; no frontend changes in this backend task.
