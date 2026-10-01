# Latest-schema backup fixture repair

Original product CI: run 36921604646 / job 110568760131 at `95b4d8e`; `infra/runtime/backup.test.mjs:48` applied current roles to a fixture containing only migrations 00001 and 00002, so `personnel.drafts` did not exist. Root explicitly coordinated and authorized this additional task path.

The unchanged fixture was run in a new dedicated PostgreSQL18.6 container with no published port and separate private credentials/volume. Each test invocation creates new source/target databases with its own timestamp. RED: three passed, the ledger-role test failed with the exact missing-relation error. Pre-fix source and raw output were committed in `1589ac9` before changing the backup fixture.

Minimal GREEN adds only 00003_query_drafts.sql and 00004_query_revision_writers.sql before roles.sql. The hand-built goose ledger still contains exactly `(0),(1)`; restored nextval must still be 3, and the backup role must remain unable to allocate any nextval. All four backup tests passed on newly created databases in that dedicated container. No production backup implementation or roles policy changed.

```powershell
$env:WEAVEOS_BACKUP_TEST_CONTAINER='weaveos-v010-q36-backup-1790888456074'
$env:WEAVEOS_BACKUP_TEST_USER='weaveos_test'
node --test infra/runtime/backup.test.mjs
```

Read-only search found `infra/server/deploy/personnel-upgrade.test.mjs:32` intentionally builds a 00001-only historical artifact before testing its upgrade; it is not another latest-schema backup fixture. Compatibility manifests and governance tests mention old migration paths as historical/structural data. No broad refactor or changes to these files were made.
