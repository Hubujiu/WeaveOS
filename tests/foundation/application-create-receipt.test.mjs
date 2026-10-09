import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { stripTypeScriptTypes } from 'node:module';
import test from 'node:test';

// Independent oracle: V030-012.2/.3 AP-FR-10 and B5a.4-7, carried
// forward by V030-050. These are transport/predicate tests, not real API E2E.
// Node's pinned built-in transformer loads the actual production modules;
// governance jobs need no package installation or copied validator.
async function productionModule(relative) {
 const source = await readFile(new URL(relative, import.meta.url), 'utf8');
 const javascript = stripTypeScriptTypes(source, { mode: 'transform' });
 return import('data:text/javascript;base64,' + Buffer.from(javascript).toString('base64'));
}
const { applicationApiEnvelope, ApplicationError } = await productionModule('../../apps/web/src/applications/api.ts');
const { validCreatedApplication } = await productionModule('../../apps/web/src/applications/creationReceipt.ts');
const actorId = '00000000-0000-4000-8000-000000000001';
const application = {
 id: '00000000-0000-4000-8000-000000000002',
 name: '新业务应用', ownerUserId: actorId, policyRevision: 1,
};
const operation = { name: application.name, operationId: '00000000-0000-4000-8000-000000000004' };
const transportFailures = [
 { name: '201 empty JSON', status: 201, json: {}, code: 'COMMON_SERVICE_UNAVAILABLE' },
 { name: '201 JSON null', status: 201, json: null, code: 'COMMON_SERVICE_UNAVAILABLE' },
 { name: '201 error envelope', status: 201, json: { code: 'COMMON_SERVICE_UNAVAILABLE', data: null }, code: 'COMMON_SERVICE_UNAVAILABLE' },
 { name: '201 missing data', status: 201, json: { code: 'OK' }, code: 'APPLICATION_OPERATION_UNCONFIRMED' },
 { name: '200 valid application body', status: 200, json: { code: 'OK', data: application }, code: 'APPLICATION_OPERATION_UNCONFIRMED' },
];
const invalidApplications = [
 { name: '201 malformed application', data: { id: application.id, name: application.name } },
 { name: '201 wrong owner', data: { ...application, ownerUserId: '00000000-0000-4000-8000-000000000003' } },
 { name: '201 invalid application UUID', data: { ...application, id: 'not-an-app-id' } },
 { name: '201 wrong initial revision', data: { ...application, policyRevision: 2 } },
];

function responseFixture(t, status, json) {
 const previousDocument = Object.getOwnPropertyDescriptor(globalThis, 'document');
 Object.defineProperty(globalThis, 'document', { configurable: true, value: { cookie: '' } });
 t.after(() => {
  if (previousDocument) Object.defineProperty(globalThis, 'document', previousDocument);
  else delete globalThis.document;
 });
 t.mock.method(globalThis, 'fetch', async () => new Response(JSON.stringify(json), {
  status, headers: { 'Content-Type': 'application/json' },
 }));
}

for (const reply of transportFailures) {
 test('create receipt transport classifies unconfirmed: ' + reply.name, async t => {
  responseFixture(t, reply.status, reply.json);
  await assert.rejects(applicationApiEnvelope(actorId, 'applications', 'POST', operation, undefined, 201), error => {
   assert.ok(error instanceof ApplicationError);
   assert.deepEqual({ status: error.status, code: error.code, unconfirmed: error.unconfirmed },
    { status: reply.status, code: reply.code, unconfirmed: true });
   return true;
  });
 });
}

for (const reply of invalidApplications) {
 test('create receipt entity requires a valid owned initial application: ' + reply.name, async t => {
  responseFixture(t, 201, { code: 'OK', data: reply.data });
  const receipt = await applicationApiEnvelope(actorId, 'applications', 'POST', operation, undefined, 201);
  assert.deepEqual(receipt, { data: reply.data, meta: null, location: null });
  assert.equal(validCreatedApplication(receipt.data, actorId), false);
 });
}

test('create receipt accepts the exact successful 201 owned initial application', async t => {
 responseFixture(t, 201, { code: 'OK', data: application });
 const receipt = await applicationApiEnvelope(actorId, 'applications', 'POST', operation, undefined, 201);
 assert.deepEqual(receipt, { data: application, meta: null, location: null });
 assert.equal(validCreatedApplication(receipt.data, actorId), true);
});
