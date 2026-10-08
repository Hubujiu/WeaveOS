import assert from 'node:assert/strict';

// JSON Schema required/enum/composition alternatives are unordered. Values
// inside an enum still retain their own array order and exact structure.
function canonical(value) {
  if (Array.isArray(value)) return value.map(canonical);
  if (value && typeof value === 'object') return Object.fromEntries(Object.keys(value).sort().map(key => [key, canonical(value[key])]));
  return value;
}
export function assertUnordered(actual, expected, message) {
  assert.ok(Array.isArray(actual), message ?? 'expected an actual array');
  assert.ok(Array.isArray(expected), 'expected an independent array oracle');
  const encode = values => values.map(value => JSON.stringify(canonical(value))).sort();
  assert.deepEqual(encode(actual), encode(expected), message);
}
