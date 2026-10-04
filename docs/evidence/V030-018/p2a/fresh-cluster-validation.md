# P2a fresh-cluster migration validation

- Fixture: disposable PostgreSQL `18.6`, started with `--network none`, `listen_addresses=''`, and a dedicated Unix socket at `/tmp/v030-018-fresh-pg-PQ4m7C`. Before migration, `pg_roles` had zero roles matching `auth_%`.
- Applied `goose -dir db/migrations postgres 'host=/tmp/v030-018-fresh-pg-PQ4m7C user=postgres dbname=v018_fresh sslmode=disable' up`. It reached version 13 successfully before `roles.sql` was applied. Raw result: `fresh-cluster-migrations-up.log`; pre/post role counts and migration version: `fresh-cluster-auth-roles-before.txt`, `fresh-cluster-after-up-state.txt`.
- Applied `infra/runtime/roles.sql` with `docker exec -i ... psql -X -U postgres -d v018_fresh -v ON_ERROR_STOP=1`. The role privilege catalog shows `auth_app` has EXECUTE on Acquire/Release, but no fence INSERT/UPDATE/DELETE or sequence USAGE/SELECT/UPDATE; PUBLIC has no EXECUTE on either function.
- As `auth_app`, attempted `SELECT nextval('applications.record_command_fence_epoch_seq')`, direct fence INSERT, UPDATE, and DELETE. All four exited 1 with permission denied. Results: `fresh-sequence-direct-write.log`, `fresh-fence-{insert,update,delete}.log` and matching `.exit-code` files.
- Ran the 14 Root tests and package regressions against this cluster after role installation. Redis fixture used the pinned image digest `sha256:164c759a0c342ee69d08fc99219382b0fd682181465c0df2e0e6911f4c85d73c`; appstructure also used a separately migrated archive database with `cold-roles.sql`.

No test source or test logic changed for this fix. The pre-existing PR30 CI failure IDs are recorded in `docs/tasks/V030-018.md`.
