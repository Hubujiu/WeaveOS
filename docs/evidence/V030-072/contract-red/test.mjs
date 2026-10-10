import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import assert from 'node:assert/strict';

const api = JSON.parse(readFileSync(new URL('./openapi/openapi.json', import.meta.url)));
const path = '/api/v1/applications/{appId}/forms/{viewId}/records/{recordId}';
test('approved lifecycle commands expose bounded CAS input and minimal confirmed output', () => {
  for (const action of ['deletion', 'restoration']) {
    const operation = api.paths[`${path}/${action}`]?.post;
    assert.ok(operation, `${action} must be a documented command`);
    assert.ok(operation.responses['200']);
    assert.ok(operation.responses['409']);
    assert.ok(operation.responses['503']);
  }
  const request = api.components.schemas.RecordLifecycleRequest;
  assert.ok(request);
  assert.equal(request.additionalProperties, false);
  assert.deepEqual([...request.required].sort(), ['expectedRecordVersion', 'expectedSchemaVersion', 'operationId']);
  assert.deepEqual(Object.keys(request.properties).sort(), ['expectedRecordVersion', 'expectedSchemaVersion', 'operationId']);
  const result = api.components.schemas.RecordLifecycleResult;
  assert.ok(result);
  assert.equal(result.additionalProperties, false);
  assert.deepEqual([...result.required].sort(), ['deleted', 'id', 'operationId', 'recordVersion', 'schemaVersion']);
  assert.deepEqual(Object.keys(result.properties).sort(), ['deleted', 'id', 'operationId', 'recordVersion', 'schemaVersion']);
});
test('lifecycle recovery state is documented without business values', () => {
  assert.ok(api.paths[`${path}/lifecycle`]?.get);
  const state = api.components.schemas.RecordLifecycleState;
  assert.ok(state);
  assert.equal(state.additionalProperties, false);
  assert.deepEqual(Object.keys(state.properties).sort(), ['deleted', 'id', 'recordVersion', 'schemaVersion']);
});
