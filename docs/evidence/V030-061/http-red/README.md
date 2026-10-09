# P2 original RED

Actual Node contract: two failures because missing global inbox route and item schema; contract.exit=1.
Actual TLS host: both new top-level tests and four ingress subcases see404 API_NOT_FOUND on the unimplemented exact route, before business/ingress expected assertions. Original JSON log ../inbox-http-red, source819dd0a, exit1. No setup/compile failure.
Review before implementation corrected a later (not yet reached) test preview locator to the already frozen GET task resource without /preview. Existing task API not changed. All new route/field/auth/error assertions retained.
