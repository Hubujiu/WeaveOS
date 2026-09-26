// Test oracle only: the finite JSON Schema vocabulary used by response DTOs.
// Unknown constraints fail loudly; this is not an application validator.
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
const document = JSON.parse(readFileSync(new URL('../../contracts/openapi/openapi.json', import.meta.url)));
const dereference = ref => ref.slice(2).split('/').reduce((value, key) => value[key], document);
function matches(schema, value) {
  for (const key of Object.keys(schema)) assert.ok(['$ref','type','required','additionalProperties','properties','const','enum','minLength','minItems','items','allOf','oneOf','description','example'].includes(key), `Unsupported response constraint ${key}`);
  if (schema.$ref && !matches(dereference(schema.$ref), value)) return false;
  if (schema.allOf && !schema.allOf.every(child => matches(child, value))) return false;
  if (schema.oneOf && schema.oneOf.filter(child => matches(child, value)).length !== 1) return false;
  const type = value === null ? 'null' : Array.isArray(value) ? 'array' : typeof value;
  if (schema.type && ![schema.type].flat().includes(type)) return false;
  if ('const' in schema && value !== schema.const) return false;
  if (schema.enum && !schema.enum.includes(value)) return false;
  if (type === 'string' && [...value].length < (schema.minLength ?? 0)) return false;
  if (type === 'array' && (value.length < (schema.minItems ?? 0) || schema.items && !value.every(item => matches(schema.items, item)))) return false;
  if (type === 'object') {
    if (schema.required?.some(key => !(key in value))) return false;
    if (schema.additionalProperties === false && Object.keys(value).some(key => !(key in (schema.properties ?? {})))) return false;
    if (Object.entries(schema.properties ?? {}).some(([key, child]) => key in value && !matches(child, value[key]))) return false;
  }
  return true;
}
export async function assertResponseSchema(response, path, method) {
  const template = path.replace(/\/users\/[^/]+\/password-reset$/, '/users/{userId}/password-reset');
  let schema = document.paths[template]?.[method.toLowerCase()]?.responses[String(response.status)];
  if (!schema && response.status === 404) schema = { content: { 'application/json': { schema: { $ref: '#/components/schemas/ErrorEnvelope' } } } };
  assert.ok(schema, `Undeclared HTTP status ${response.status} for ${method} ${template}`);
  if (schema.$ref) schema = dereference(schema.$ref);
  if (response.status === 401) assert.equal(response.headers.get('www-authenticate'), 'Session realm="enterprise-management-system"');
  if (method === 'HEAD' || response.status === 204) { assert.equal(await response.clone().text(), ''); return; }
  assert.ok(response.headers.get('content-type')?.includes('application/json'));
  const body = await response.clone().json();
  assert.ok(body.message && body.message !== body.code, 'Public message must be human-readable');
  assert.equal(matches(schema.content['application/json'].schema, body), true, `Response fails ${method} ${template} status ${response.status} schema`);
  assert.equal(body.meta?.requestId, response.headers.get('x-request-id'));
  assert.ok(response.headers.get('cache-control')?.includes('no-store'));
}
