# Corrected P2d RED environment

- Go: 1.27.1 (CI-pinned `.go-version`).
- PostgreSQL: isolated PostgreSQL 18.6 test database, goose migrations 1–15 and runtime roles installed.
- Redis: isolated Redis 8.2.10, database 15.
- Execution: Go 1.27.1 container with the isolated PostgreSQL and Redis services; temporary Go trust bundle included the environment proxy CA so module download completed.
- Root test SHA-256 immediately before and after RED is recorded in `root-test-corrected-pre-red.sha256` and `root-test-corrected-post-red.sha256`.
- Raw test output and exit code: `corrected-red-output.txt` and `corrected-red-exit.txt`.
