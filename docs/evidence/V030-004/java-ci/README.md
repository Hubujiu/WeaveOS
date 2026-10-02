# B3 Java CI gate evidence

Oracle: PRD/ADR009 Java-specific CI PLAN read back at source versions 2026-10-02T14:44:49.941Z / 14:44:54.611Z. Root released the write gate and assigned only the independent Java workflow plus this package's runner/report validation/tests and task scope/evidence. Related PR paths and manual dispatch only; no deployment integration, new secrets or permission expansion.

Original CI-gate RED: 2026-10-02T14:50:04.430230Z, `python3 prototypes/flowable-local-tx/ci_tests.py`, exit 1; 25 tests, 23 assertion failures, 0 errors. The declarations load correctly and intentionally have no behavior. Failures demonstrate absent rejection of failed/skipped/missing/stale/subset/malformed reports, mismatched exit codes, missing runner verdicts and missing workflow wiring. The passing positive report fixture and actual intentionally failing assertion subprocess are controls, not Java/PG evidence.

`red-source.tar.gz` preserves the exact test/manifest/stub sources before implementation; its SHA256 is in `red-result.json`. `red-output.txt` contains original output, not a reconstruction. The existing Java suite and original earlier RED/GREEN artifacts are unchanged. CI fixtures cannot replace the real 15-test PostgreSQL run.
