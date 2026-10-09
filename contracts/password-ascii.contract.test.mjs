import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import Ajv2020 from 'ajv/dist/2020.js';
const api = JSON.parse(readFileSync(new URL('./openapi/openapi.json', import.meta.url)));
const properties = path => api.paths[path].post.requestBody.content['application/json'].schema.properties;
test('Q13: registration schema accepts only printable ASCII with four classes', () => {
  // Q13 supplies the independent fixtures; validate the published Schema Object.
  // OAS password is a UI annotation, not a complexity or type validator.
  const accepts = new Ajv2020({
    strict: true, formats: { password: true },
    coerceTypes: false, useDefaults: false, removeAdditional: false,
  }).compile(properties('/api/v1/registrations').password);
  for (const [value, expected] of [['Aa1!', true], [' Aa1! ', true], ['Aa1~', true], ['Aa1!中', false], ['Aa1!\t', false], ['Aa1!\n', false], ['Aa1!\x7f', false], ['Aa1 ', false], ['aa1!', false], ['AA1!', false], ['Aa!!', false], [['A', 'a', '1', '!'], false], [null, false], [true, false], [1234, false], [{}, false]]) {
    const original = structuredClone(value);
    assert.equal(accepts(value), expected, 'approved synthetic password fixture must match schema');
    assert.deepEqual(value, original, 'schema validation must not rewrite input');
  }
});
test('approved account length applies after ordinary-space trimming, not to raw input', () => {
  for (const path of ['/api/v1/registrations', '/api/v1/sessions']) {
    const account = properties(path).account;
    assert.equal(account.maxLength, undefined, 'raw length cannot reject approved edge-space normalization');
    assert.equal(account['x-max-length-after-trim'], 254);
    assert.match(account.description, /ordinary.*space/i);
  }
  assert.equal(properties('/api/v1/registrations').account['x-account-normalization'].rejectInternalOrdinarySpaces, true);
});
