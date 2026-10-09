import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
const api=JSON.parse(readFileSync(new URL('./openapi/openapi.json',import.meta.url)));
const path='/api/v1/applications/{appId}/forms/{viewId}/records/{recordId}/workflow-events';
test('confirmed history list is authenticated GET with closed cursor parameters',()=>{
 const p=api.paths[path];assert.ok(p,'history list route missing');assert.deepEqual(Object.keys(p),['get']);const op=p.get;
 assert.deepEqual(op.security,[{WebSession:[]}]);assert.ok(op.parameters.some(p=>p.$ref==='#/components/parameters/ExpectedActor'));
 const query=op.parameters.filter(p=>p.in==='query');assert.deepEqual(query.map(p=>p.name).sort(),['pageSize','pageToken']);
 const size=query.find(p=>p.name==='pageSize').schema;assert.equal(size.minimum,1);assert.equal(size.maximum,100);assert.equal(size.default,20);
 assert.equal(op.requestBody,undefined);assert.equal(op.responses['200'].content['application/json'].schema.$ref,'#/components/schemas/WorkflowEventHistoryEnvelope');
});
test('confirmed history list has safe bounded summaries and cursor metadata without total',()=>{
 const s=api.components.schemas.WorkflowEventHistoryItems;assert.ok(s,'list schema missing');assert.deepEqual(s.required,['items']);assert.equal(s.additionalProperties,false);assert.deepEqual(Object.keys(s.properties),['items']);assert.equal(s.properties.items.maxItems,100);assert.equal(s.properties.items.items.$ref,'#/components/schemas/WorkflowEventSummary');
 const envelope=api.components.schemas.WorkflowEventHistoryEnvelope;assert.ok(envelope);assert.equal(envelope.allOf[0].$ref,'#/components/schemas/Envelope');
 const meta=envelope.allOf[1].properties.meta;assert.ok(meta.required.includes('pagination'));assert.equal(meta.properties.pagination.$ref,'#/components/schemas/CursorPagination');
});
