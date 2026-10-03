# R13 secret-scan review

Root inspected the source of the single generic-api-key finding in adr14-wire-target-red.txt:9. The matched field is meta.pagination.nextPageToken in a failed isolated history HTTP test response. record_history_http.go creates it from 32 crypto/rand bytes and stores a session-, actor-, and resource-bound Redis cursor for ten minutes. record_history_http_test.go uses the isolated fixture namespace and real test Redis; this is not an API credential or signing key.

The persisted RED evidence now replaces only that cursor value with [REDACTED_TEST_CURSOR]. Failure names, assertion messages, business data, versions, times and exit results remain. This is disclosure sanitization, not a re-run or a rewritten GREEN result. The Git history is not rewritten.

Root also changed the existing failing assertion's diagnostic message to report only its three asserted field facts instead of the complete response body. The assertion predicate and all test behavior remain unchanged. No scanner rule, allowlist, CI gate or security configuration was weakened.

Re-run of the existing tracked-source Gitleaks test is pending at this documentation commit; earlier product run 37138046587 failed the scanner and skipped downstream runtime/restore/deployment-package gates. Ordinary CI and that run's migrations/HTTPS acceptance step passed. This review does not approve merge or deployment.
