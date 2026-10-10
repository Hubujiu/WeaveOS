import Ajv2020 from 'ajv/dist/2020.js';
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

const validator = new Ajv2020({ strict: false, validateFormats: false });
validator.addSchema({ ...api, $id: 'lifecycle' });
const check = name => validator.compile({ $ref: `lifecycle#/components/schemas/${name}` });
test('lifecycle public schemas reject independent malformed values and payload leakage', () => {
  const accepts = check('RecordLifecycleResult');
  const result = { operationId: '10000000-0000-4000-8000-000000000001', id: '20000000-0000-4000-8000-000000000001', recordVersion: 2, schemaVersion: 1, deleted: true };
  assert.equal(accepts(result), true);
  for (const bad of [{ ...result, values: {} }, { ...result, deleted: 'true' }, { ...result, recordVersion: 0 }, { ...result, recordVersion: 1.5 }, { ...result, recordVersion: 9007199254740992 }, { ...result, schemaVersion: -1 }]) assert.equal(accepts(bad), false, JSON.stringify(bad));
  for (const key of Object.keys(result)) { const bad = { ...result }; delete bad[key]; assert.equal(accepts(bad), false, key); }
  const request = check('RecordLifecycleRequest');
  const valid = { operationId: result.operationId, expectedRecordVersion: 1, expectedSchemaVersion: 1 };
  assert.equal(request(valid), true);
  for (const bad of [{ ...valid, force: true }, { ...valid, expectedRecordVersion: null }, { ...valid, expectedRecordVersion: 0 }, { ...valid, expectedSchemaVersion: -1 }, { ...valid, expectedRecordVersion: 9007199254740992 }]) assert.equal(request(bad), false);
});

test('lifecycle read documents standard HEAD without a response body', () => {
  const head = api.paths[`${path}/lifecycle`]?.head;
  assert.ok(head, 'implemented HEAD must be in the public contract');
  assert.equal(head.responses['200'].content, undefined);
  assert.deepEqual(head.security, [{ WebSession: [] }]);
  assert.ok(head.parameters.some(p => p.$ref === '#/components/parameters/ExpectedActor'));
});
test('lifecycle minimal operation recovery remains closed', () => {
  const accepts = check('ApplicationOperationEnvelope');
  const operationId = '10000000-0000-4000-8000-000000000001';
  const result = { operationId, id:'20000000-0000-4000-8000-000000000001',recordVersion:2,schemaVersion:1,deleted:true };
  const wrap = result => ({code:'OK',message:'success',meta:{requestId:'a'.repeat(32)},data:{operationId,status:'confirmed',httpStatus:200,location:'',result}});
  assert.equal(accepts(wrap(result)),true,JSON.stringify(accepts.errors));
  assert.equal(accepts(wrap({...result,values:{private:'never'}})),false);
});
