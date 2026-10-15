// Test-only diagnostics. Never return raw engine errors, URLs or request data.
const failures = new Map([
 ['Load failed','load-failed'],
 ['cancelled','cancelled'],
 ['net::ERR_ABORTED','cancelled'],
 ['net::ERR_CONNECTION_RESET','connection-reset'],
 ['net::ERR_CONNECTION_REFUSED','connection-refused'],
 ['Timeout was reached','timeout'],
 ['net::ERR_TIMED_OUT','timeout'],
]);
export function safeNetworkFailure(message) { return failures.get(message) ?? 'unknown'; }
