# Backup fixture correction

Original exact eb0 CI job114139478584 reached PostgreSQL and failed42P01 installing current roles against its frozen1..21 schema. All three other encrypted backup cases passed. The original test source and failure are preserved.

Only nine fixture lines were added: comment and explicit ordered migrations22..29, each in a transaction because LOCK TABLE migrations require it. No backup implementation, production schema, role grants, old expected result, test timeout or assertion changed. The new migration-list guard first failed for missing22 and then passed.

Native PG18.6 independent fresh databases reproduced original21+roles42P01 and corrected29+roles success. Exact bounded privilege result t|f|f|t verifies backup SELECT/no DELETE, runtime no DELETE/explicit cleanup EXECUTE. This is SQL integration evidence only; actual Docker encrypted-backup test is pending final exact CI.
