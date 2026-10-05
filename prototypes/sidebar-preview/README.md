# UI-031 HTML concept preview

This standalone demonstration renders Root's original HTML using Firefox with default security settings. It contains sample data and does not connect to a database or application service.

Sources read on 2026-10-05 UTC:

- [PRD UI-031](https://app.notion.com/p/3f02f5a9e648814b9d3eee04f5993faf)
- [ADR UI-031](https://app.notion.com/p/3f02f5a9e648810aaf38dad44925163a)

Owner: Root; execution: delegated renderer. Authorized scope: only this directory and the new sidebar-preview workflow. No PR, merge, deployment, product code edits, or existing workflow changes.

Original HTML SHA256: `9ad98e9473b66001d570bac562b3c45a43773575ee0f10a6de493c951e107765`.

The rendering script and assertions are supplied by Root. They check reversible sidebar collapse, search results (2 rows for 梁宇凡 and 10 after clearing), dialog open/cancel, page errors, and horizontal overflow at a 1440×960 viewport with device scale factor 2.

Local rendering: BLOCKED. Chromium rejected the environment's SUID sandbox configuration; default Firefox content processes failed with `Sandbox: writing /proc/self/uid_map: EROFS`. No safety settings were disabled. These environment failures are not TDD RED or product acceptance evidence.

CI status at initial push: pending. Expected artifacts: sidebar-open.png, sidebar-closed.png, and checks.json. Failure must be reported without weakening Root's assertions. This branch is retained for review and is not product delivery or acceptance.
