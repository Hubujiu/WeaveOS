# Recoverable Root RED sources

These byte-exact snapshots were recovered on 2026-10-04 from the verified task commits; this is the snapshot creation time, not a fabricated RED execution time. The original RED executions, timestamps, commands, environment, exit codes and hashes remain in the adjacent root-workflow-http-red* and audit-*-red* evidence.

The four management HTTP files come from initial Root commit d557260: the 503 appworkflows placeholder, applications handler with declaration only, original composition root, and Root's independent HTTP test. Together they preserve the initial tested source without depending on later task-branch retention. The exact audit Root test comes from 4724b87; the corrected archive fixture comes from Root correction 96e044b (id/occurred_at only). No assertions were rewritten.

Files use .txt so these evidence snapshots are not compiled as additional tests. snapshots.sha256 records their bytes. Current Root tests are equal to their authorized sources after mechanical gofmt, as recorded in merged-root-http-format-proof.txt and final-regression.md.
