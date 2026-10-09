# Original P1 RED

2026-10-09 12:43 UTC, source d8f6402, real PostgreSQL18.6 / Redis8.2.10 with restricted auth_app.
`bash /workspace/scratch/75c2273f6a80/run-v061-tests.sh inbox-red '^TestRootWorkflowInbox' ./internal/apprecordservice`
Exit1: 7 top-level, 4 subcases reached declaration-only SearchWorkflowInbox returning ErrUnavailable. No setup/compile failures; no skips. Raw original output ../inbox-red. Deeper assertions behind first successful read are not yet independently exercised; GREEN must reach them. Declaration and test source preserved before implementation.
