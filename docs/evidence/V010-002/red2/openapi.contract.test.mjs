import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { test } from 'node:test';

// Oracles: v0.1.0 PRD FR-001..018 and accepted ADR-001/002 §§5.1, 5.2, 5.6, 5.8.
// An absent document is an empty, unimplemented contract so RED reaches a behavioral assertion.
const file = new URL('./openapi/openapi.json', import.meta.url);
const api = existsSync(file) ? JSON.parse(readFileSync(file, 'utf8')) : {};
const operation = (path, method) => api.paths?.[path]?.[method];

test('API-01: contract is OpenAPI 3.2.1 with the five authentication routes', () => {
  assert.equal(api.openapi, '3.2.1');
  for (const [path, methods] of [
    ['/api/v1/registrations', ['post']],
    ['/api/v1/sessions', ['post']],
    ['/api/v1/sessions/current', ['get', 'head', 'delete']],
    ['/api/v1/invitations', ['post']],
    ['/api/v1/users/{userId}/password-reset', ['post']],
  ]) for (const method of methods) assert.ok(operation(path, method), `${method.toUpperCase()} ${path} must be described`);
});

test('API-02: public writes and protected Cookie security match ADR-001/002', () => {
  assert.deepEqual(operation('/api/v1/registrations', 'post')?.security, []);
  assert.deepEqual(operation('/api/v1/sessions', 'post')?.security, []);
  assert.deepEqual(api.components?.securitySchemes?.WebSession, {
    type: 'apiKey', in: 'cookie', name: '__Host-weaveos_session',
  });
  for (const [path, method] of [
    ['/api/v1/sessions/current', 'get'], ['/api/v1/sessions/current', 'delete'],
    ['/api/v1/invitations', 'post'], ['/api/v1/users/{userId}/password-reset', 'post'],
  ]) assert.deepEqual(operation(path, method)?.security, [{ WebSession: [] }]);
});

test('API-03: creation, current session, logout and reset have precise statuses', () => {
  for (const [path, method, status] of [
    ['/api/v1/registrations', 'post', '201'], ['/api/v1/sessions', 'post', '201'],
    ['/api/v1/sessions/current', 'get', '200'], ['/api/v1/sessions/current', 'head', '200'],
    ['/api/v1/sessions/current', 'delete', '204'], ['/api/v1/invitations', 'post', '201'],
    ['/api/v1/users/{userId}/password-reset', 'post', '200'],
  ]) assert.ok(operation(path, method)?.responses?.[status], `${method.toUpperCase()} ${path}: ${status}`);
  assert.equal(operation('/api/v1/sessions/current', 'delete')?.responses?.['204']?.content, undefined);
  assert.equal(operation('/api/v1/sessions/current', 'head')?.responses?.['200']?.content, undefined);
});

test('API-04: JSON envelope has exact required keys and no arbitrary top-level data', () => {
  const envelope = api.components?.schemas?.Envelope;
  assert.deepEqual(envelope?.required?.slice().sort(), ['code', 'data', 'message', 'meta']);
  assert.equal(envelope?.additionalProperties, false);
  assert.ok(envelope?.properties?.data);
  assert.ok(envelope?.properties?.meta);
});

test('API-05: registration and login require account/password; registration requires invitation', () => {
  for (const [path, required] of [
    ['/api/v1/registrations', ['account', 'password', 'invitationCode']],
    ['/api/v1/sessions', ['account', 'password']],
  ]) {
    const body = operation(path, 'post')?.requestBody;
    assert.deepEqual(Object.keys(body?.content ?? {}), ['application/json']);
    assert.deepEqual(body.content['application/json'].schema.required, required);
  }
});

test('API-06: published error table preserves accepted ADR mapping', () => {
  const file = new URL('./errors/codes.json', import.meta.url);
  const codes = existsSync(file) ? JSON.parse(readFileSync(file, 'utf8')) : {};
  for (const [code, status] of [
    ['COMMON_VALIDATION_FAILED', 400], ['AUTH_INVALID_CREDENTIALS', 401],
    ['AUTH_UNAUTHENTICATED', 401], ['COMMON_PERMISSION_DENIED', 403],
    ['INVITATION_INVALID', 400], ['INVITATION_ALREADY_USED', 409],
    ['USER_ACCOUNT_ALREADY_EXISTS', 409], ['COMMON_SERVICE_UNAVAILABLE', 503],
  ]) assert.equal(codes[code]?.httpStatus, status, `${code} HTTP mapping`);
});

test('API-07: account rule preserves case and rejects internal whitespace after trimming', () => {
  const account = operation('/api/v1/registrations', 'post')?.requestBody?.content?.['application/json']?.schema?.properties?.account;
  assert.deepEqual(account?.['x-account-normalization'], {
    trimEdgeSpaces: true, caseFolding: false, rejectInternalWhitespace: true,
  });
});

test('API-08: every declared response carries a server-issued request ID', () => {
  for (const [path, item] of Object.entries(api.paths ?? {})) {
    for (const [method, op] of Object.entries(item)) {
      for (const [status, raw] of Object.entries(op.responses ?? {})) {
        const response = raw.$ref ? api.components.responses[raw.$ref.split('/').at(-1)] : raw;
        assert.ok(response.headers?.['X-Request-Id'], `${method.toUpperCase()} ${path} ${status} needs X-Request-Id`);
      }
    }
  }
});
