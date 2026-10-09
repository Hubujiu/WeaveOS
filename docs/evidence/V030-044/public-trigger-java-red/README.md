# Public trigger → actual Flowable: genuine RED, not delivered

Source before test: remote checkpoint 5867373615f81d7d6c7b5b58cc03e40922850c63.
Test-first commit: 2487111 (full immutable SHA in source-head.txt).
Requirement: V044 PRD and ADR require durable accepted trigger intents to be dispatched to the published workflow; successful record submission must not be described as engine startup.

The isolated native harness used PostgreSQL 18.6, Redis 8.2.10, Go 1.27.2 and official Adoptium JDK17.0.20+101. Java source was compiled by Maven 3.9.11 against repository dependencies. NativeExecutionRpcFixtureMain (copied here) uses loopback-only gRPC and a fresh synthetic PostgreSQL schema. It is a distinct harness; the Docker fixture's hostname assertions were not removed or weakened. No production deployment or credentials are involved.

Observed: actual HTTPS workflow definition creation, genuine Go→Java deployment RPC, enable, and actual HTTPS record Create all succeed. The public write produces one durable starting instance. Draining the existing execution worker returns idle; the instance remains starting instead of active. The failing assertion is the missing admission/dispatch behavior, not a compilation, network or fixture failure. No ReserveInTx / AcceptExecutionInTx / fence injection is used to manufacture a successful start.

This test remains FAIL, not skipped. It is locally committed before implementation. It is not part of the earlier remote checkpoint's green claims. Starting-intent admission implementation remains dependent on the unresolved technical-contract publication permission; no denied Notion write was retried. Additional recovery and permission tests remain necessary.
