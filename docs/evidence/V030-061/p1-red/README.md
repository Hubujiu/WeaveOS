# Original P1 RED

2026-10-09 12:43 UTC, source d8f6402, real PostgreSQL18.6 / Redis8.2.10 with restricted auth_app.
`bash /workspace/scratch/75c2273f6a80/run-v061-tests.sh inbox-red '^TestRootWorkflowInbox' ./internal/apprecordservice`
Exit1: 7 top-level, 4 subcases reached declaration-only SearchWorkflowInbox returning ErrUnavailable. No setup/compile failures; no skips. Raw original output ../inbox-red. Deeper assertions behind first successful read are not yet independently exercised; GREEN must reach them. Declaration and test source preserved before implementation.

First implementation run: 5 top/4 sub passed, 2 failed before their intended assertions because new tests used non-UUID SessionRef literals. Existing querycontext Prefix explicitly requires a canonical UUID; corrected only synthetic session identity to independently generated canonical UUID. Owner/non-assignee and cross-session rejection expectations unchanged. Original test/stub and failed logs preserved; no production token validation change. This is test fixture correction, not a product behavior fix.
