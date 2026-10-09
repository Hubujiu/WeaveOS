# Correct task delivery metadata, preserve failed gate

Exact remote7043defc: CI37921592153/product37921591948 failed the unchanged tasks preflight, all downstream business jobs NOT RUN. Governance37921591493 passed. Original failure is reproduced in before.log/exit1: ready task has incomplete items. The final local tasks check in the preceding archive also exited1; it was incorrectly summarized together with earlier passing checks. The prior README now explicitly corrects that claim.

Fix only metadata back to in_progress and document the remaining exact CI condition. Leave the CI acceptance item unchecked; do not relax task-policy/preflight or silently claim completion. after-tasks.log/exit0 confirms the repair. Running the unchanged full preflight now passes policy tests, structure, tasks, gofmt, contracts and offline Redocly, then fails secrets because this environment has no Docker. after-preflight.exit1 is retained and not described as full preflight GREEN. Native pinned Gitleaks scan of the exact tracked source archive passes separately; the actual Docker gate remains for CI.

No behavior/source/test-oracle changes. Genuine eight-package551/293 and Java10 results from the prior candidate remain valid for identical business source. Formal-runtime fixture correction still needs actual new-head CI, not merely compilation. No develop/main merge or deployment.
