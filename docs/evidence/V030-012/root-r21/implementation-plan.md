# Root R21: shared record-operation prerequisites

Base: f9848986d3be7c4895ca822ea8021ce882ae8fb3. Implementation model: gpt-6-luna. Root owns every test and decision; implementers run frozen tests unchanged.

## Bounded scope
This is the immediate prerequisite to mounting records in the Shell. Do not redesign the table, add workflow scope, change tests, or create a second mutation controller.

1. Extend the shared operation result with synchronous getPendingStatus(): 'preflight' | 'write_in_flight' | 'unknown' | null, derived from existing inFlight/snapshot/uncertain refs. Preflight cancellation returns null immediately. Confirmed or definite failure returns null. Never infer this solely from delayed React state.
2. Add errorCode?: string to shared operation State. On definite ApplicationError expose only its code; clear it on next operation, cancellation, confirmation, dismissal. Existing message safety stays unchanged. Do not expose response data.
3. Mutation HTTP 408 and all 5xx remain unconfirmed even when a generic JSON error was returned. Preserve the original packet and key. Read-only search 503 remains an ordinary read failure. Existing definite 400/403/409 rules and recovery uncertainty remain.
4. Restrict applicationReadPost to the exact canonical applications/<UUID>/forms/<UUID>/records/search shape, UUID structural hex with no version or variant requirement. Reject fragment, query, dot segments and encoded segment tricks before fetch. Keep the existing TypeError message.
5. LeaveScope gets only the already specified record and draft identities (Root declaration included); scopeKey includes recordId versus clientDraftId discriminator, or draftId. Keep existing structure/designer keys and instance-safe unsubscribe behavior. Do not persist unsent contents.

## Files allowed for implementation
apps/web/src/applications/useApplicationOperation.ts
apps/web/src/applications/api.ts
apps/web/src/applications/shell/leaveGuards.ts

Root has already applied the accepted type-only LeaveScope declaration in forms/leaveGuard.ts. No implementation change there is necessary.

## Tests and execution
Root's added operation-guard suite: six cases. Record-leave suite: three. Boundary suite: existing eight plus four new cases (three malformed paths and read-only 503).
Run all three suites before code changes and preserve actual results; do not count TypeScript/import failures as behavioral RED. Run again after changes, then existing root-scoped suite and related applications/records component suites, typecheck and build. Use the existing pinned Playwright 1.63 container, a fresh writable output directory, and mounted logs/output. Save the complete outer commands and exit codes; avoid a long all-project browser run until integration.
Stop after implementation and report diff, logs and any blocker; do not publish without Root review of result. No main merge or deployment.
