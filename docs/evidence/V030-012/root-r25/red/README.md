# R25 topology RED

Before implementation, `node --test tests/acceptance/topology.test.mjs` ran against Root's frozen R25 regression tests. Result: 7 passed, 2 failed, exit 1. The two failures were the missing dedicated definition key/budgets in the acceptance runner and missing definition settings in the immutable-runtime runner. The harness stops at its first OpenSSL command; it did not start Docker or services. The complete command, output and exit code are retained. Secret values are withheld by the assertions and none are logged.
