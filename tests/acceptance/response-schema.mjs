// Test oracle only: the finite JSON Schema vocabulary used by response DTOs.
// Unknown constraints fail loudly; this is not an application validator.
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
const document = JSON.parse(readFileSync(new URL('../../contracts/openapi/openapi.json', import.meta.url)));
const codes = JSON.parse(readFileSync(new URL('../../contracts/errors/codes.json', import.meta.url)));
const dereference = ref => ref.slice(2).split('/').reduce((value, key) => value[key], document);
function matches(schema, value) {
  for (const key of Object.keys(schema)) assert.ok(['$ref','type','required','additionalProperties','properties','const','enum','minLength','maxLength','minimum','maximum','pattern','format','minItems','maxItems','uniqueItems','items','allOf','oneOf','description','example','x-max-canonical-bytes'].includes(key), `Unsupported response constraint ${key}`);
  // Draft payloads contain only strings/arrays/null; compact JSON has the same
  // UTF-8 byte count regardless of object key order. Do not measure JS length.
  if (schema['x-max-canonical-bytes'] !== undefined && Buffer.byteLength(JSON.stringify(value),'utf8') > schema['x-max-canonical-bytes']) return false;
  if (schema.$ref && !matches(dereference(schema.$ref), value)) return false;
  if (schema.allOf && !schema.allOf.every(child => matches(child, value))) return false;
  if (schema.oneOf && schema.oneOf.filter(child => matches(child, value)).length !== 1) return false;
  const type = value === null ? 'null' : Array.isArray(value) ? 'array' : typeof value;
  if (schema.type && ![schema.type].flat().some(expected => expected === type || expected === 'integer' && typeof value === 'number' && Number.isInteger(value))) return false;
  if ('const' in schema && value !== schema.const) return false;
  if (schema.enum && !schema.enum.includes(value)) return false;
  if (type === 'number' && (value < (schema.minimum ?? -Infinity) || value > (schema.maximum ?? Infinity))) return false;
  if (type === 'string') {
    if (schema.pattern && !new RegExp(schema.pattern,'u').test(value)) return false;
    if ([...value].length < (schema.minLength ?? 0) || [...value].length > (schema.maxLength ?? Infinity)) return false;
    if (schema.format) {
      assert.ok(['uuid','date-time'].includes(schema.format), `Unsupported response format ${schema.format}`);
      if (schema.format === 'uuid' && !/^[a-f\d]{8}-[a-f\d]{4}-[a-f\d]{4}-[a-f\d]{4}-[a-f\d]{12}$/i.test(value)) return false;
      if (schema.format === 'date-time' && (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/i.test(value) || !Number.isFinite(Date.parse(value)))) return false;
    }
  }
  if (type === 'array' && (value.length < (schema.minItems ?? 0) || value.length > (schema.maxItems ?? Infinity) || schema.items && !value.every(item => matches(schema.items, item)))) return false;
  if (type === 'array' && schema.uniqueItems && new Set(value.map(item=>JSON.stringify(item))).size !== value.length) return false;
  if (type === 'object') {
    if (schema.required?.some(key => !(key in value))) return false;
    if (schema.additionalProperties === false && Object.keys(value).some(key => !(key in (schema.properties ?? {})))) return false;
    if (Object.entries(schema.properties ?? {}).some(([key, child]) => key in value && !matches(child, value[key]))) return false;
  }
  return true;
}
export async function assertResponseSchema(response, path, method) {
  const pathname = new URL(path, 'https://weaveos.test').pathname;
  const template = Object.keys(document.paths).find(candidate => {
    const pattern = candidate.split('/'); const actual = pathname.split('/');
    return pattern.length === actual.length && pattern.every((part, index) => /^\{[^}]+\}$/.test(part) || part === actual[index]);
  }) ?? pathname;
  let schema = document.paths[template]?.[method.toLowerCase()]?.responses[String(response.status)];
  if (!schema && response.status === 404) schema = { content: { 'application/json': { schema: { $ref: '#/components/schemas/ErrorEnvelope' } } } };
  assert.ok(schema, `Undeclared HTTP status ${response.status} for ${method} ${template}`);
  if (schema.$ref) schema = dereference(schema.$ref);
  if (response.status === 401) assert.equal(response.headers.get('www-authenticate'), 'Session realm="enterprise-management-system"');
  if (method === 'HEAD' || response.status === 204) { assert.equal(await response.clone().text(), ''); return; }
  assert.ok(response.headers.get('content-type')?.includes('application/json'));
  const body = await response.clone().json();
  assert.ok(body.message && body.message !== body.code, 'Public message must be human-readable');
  if (response.status >= 400) {
    // API_NOT_FOUND is the foundation host's generic API404, not a user-resource404.
    assert.equal(body.code === 'API_NOT_FOUND' ? 404 : codes[body.code]?.httpStatus, response.status, 'Actual business code must match its registered HTTP status');
  } else assert.equal(body.code, 'OK');
  assert.equal(matches(schema.content['application/json'].schema, body), true, `Response fails ${method} ${template} status ${response.status} schema`);
  assert.equal(body.meta?.requestId, response.headers.get('x-request-id'));
  assert.ok(response.headers.get('cache-control')?.includes('no-store'));
}
