// Interface-only RED scaffold. No validation or cleanup is implemented.
export function parseTask(_text) { return null; }
export function validateTask(_text) { return []; }
export function evaluateCleanup(_input) { return { allowed: false, reasons: ['not implemented'] }; }
