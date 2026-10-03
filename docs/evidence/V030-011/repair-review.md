# B5a root review repair

Root has not accepted ff88ec86cd3fb0d2227baec57b99085b819b017d.
Its P1/P2 findings reopen the local delivery, within the unchanged B5a freeze.
PRD and ADR009 were fetched again; timestamps remain 2026-10-03T02:21:51.820Z
and 2026-10-03T02:21:59.281Z, respectively. All ten clauses were reread.

P1: an actual successful PostgreSQL COMMIT followed by a synthetic wire
ErrorResponse with SQLSTATE 40003 or 23514 must report UNCONFIRMED. Only
pgx.ErrTxCommitRollback is positive rollback evidence here. Preserve the
original internal cause, disclose none of it in HTTP, never auto-retry, and
recover with the same actor/key without another audit or policy revision.

P2: List must use the trusted actor query plus one authorized set query in
one RR transaction. Validate actual menu/catalog and complete grant tuples
with current same-app membership and enabled group; retain stable ordering
and empty arrays. Member existence and replacement, and grant validation and
replacement, must use sets. No permission cache, pagination or truncation.

Actual RED: repair-commit-red (exit 1) includes four successful-commit HTTP
cases, lost private-cause assertion and a passing explicit rollback negative.
repair-cost-red (exit 1) uses the same isolated PG18.6: List at A=10/100/1000
uses 32/302/3002 statements; member replacement at M=10/100/1000 uses
37/217/2017 statements; a valid plus unknown complete grant set uses two
validation queries rather than one. These are runtime behavior failures,
not compiler/environment failures. Exact prior store/tests are archived in
repair-red-source.tar.gz before any repair implementation.

The SQL tracer excludes transaction control and SET ROLE, never logs values.
Scale fixtures explicitly reset only the seven application tables and their
synthetic catalogue entries in the known isolated test DB. No production,
remote push, new PR, original PR21 edit, main merge or deployment is authorized.
