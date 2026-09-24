export function releaseErrors(matrix) {
  if (!matrix || matrix.version !== '0.1.0' || !Array.isArray(matrix.checks) || !matrix.checks.length) return ['[RELEASE] nonempty v0.1.0 acceptance matrix required'];
  const errors = [], ids = new Set();
  for (const check of matrix.checks) {
    if (!check || typeof check.id !== 'string' || ids.has(check.id)) { errors.push('[RELEASE] missing or duplicate check id'); continue; }
    ids.add(check.id);
    if (!['automated', 'manual'].includes(check.mode)) errors.push(`[RELEASE] ${check.id}: invalid mode`);
    if (check.status !== 'passed') errors.push(`[RELEASE] ${check.id}: ${check.status ?? 'missing status'}`);
    for (const field of ['owner', 'evidence']) if (typeof check[field] !== 'string' || !check[field].trim()) errors.push(`[RELEASE] ${check.id}: missing ${field}`);
  }
  return errors;
}
