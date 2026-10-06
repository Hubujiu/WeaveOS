# Flowable 8.0.0 native PostgreSQL SQL sources

These three SQL files are unmodified resources from the official Flowable
8.0.0 source artifacts published by Flowable through Maven Central. Their
original bytes, resource paths, artifact URLs, sizes and SHA-256 values are
recorded in `manifest.json`. The fixed order is common, engine, history.

Flowable is the upstream project and organization. The bundled
`flowable-root-8.0.0.pom` declares its license as Apache v2 in its `licenses`
section. `LICENSE.txt` contains the Apache License, Version 2.0, obtained
unchanged from the Apache Software Foundation's official license URL.

The Maven parent chain for both SQL-bearing artifacts is:

1. `org.flowable:flowable-engine-common:8.0.0` or
   `org.flowable:flowable-engine:8.0.0`
2. `org.flowable:flowable-parent:8.0.0`
3. `org.flowable:flowable-dependencies:8.0.0`
4. `org.flowable:flowable-bom:8.0.0`
5. `org.flowable:flowable-root:8.0.0`

The reviewed official source jars contain Apache-2.0 source-code license
headers, but do not provide a separate LICENSE or NOTICE resource. The SQL
resources themselves have no license header; none has been inserted, so their
upstream bytes remain intact. This attribution and license evidence accompany
the originals without inventing an upstream NOTICE.

This bundle preserves sources for offline verification. It does not execute a
database migration, create roles, or establish that a database has this schema.
