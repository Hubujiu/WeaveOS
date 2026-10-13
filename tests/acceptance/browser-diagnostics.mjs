// Test-only diagnostics. Never return raw engine errors, URLs or request data.
export function safeNetworkFailure(_message) { return 'unknown'; }
