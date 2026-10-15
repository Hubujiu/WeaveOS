import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
const api=JSON.parse(readFileSync(new URL('./openapi/openapi.json',import.meta.url)));
const path='/api/v1/applications/{appId}/forms/{viewId}/records/{recordId}/workflow-instances/{instanceId}/tasks/{taskId}/reference-candidates';
test('V075 task candidates have an independent five-resource GET with current identity',()=>{
 const route=api.paths[path];assert.ok(route,'trusted task candidate endpoint missing');assert.deepEqual(Object.keys(route),['get']);
 assert.deepEqual(route.get.security,[{WebSession:[]}]);assert.ok(route.get.parameters.some(p=>p.$ref==='#/components/parameters/ExpectedActor'));
 assert.deepEqual(route.get.parameters.filter(p=>p.in==='path').map(p=>p.name),['appId','viewId','recordId','instanceId','taskId']);
 assert.equal(route.get.requestBody,undefined);
});
test('V075 task candidates use bounded field/query/cursor only, never a claimed task authority',()=>{
 const op=api.paths[path]?.get;assert.ok(op,'task candidates GET missing');
 const query=op.parameters.filter(p=>p.in==='query');assert.deepEqual(query.map(p=>p.name),['fieldId','q','pageSize','pageToken']);
 assert.equal(query[0].required,true);assert.equal(query[1].schema.maxLength,100);
 assert.deepEqual(query[2].schema,{type:'integer',minimum:1,maximum:50,default:20});
 assert.equal(query[3].schema.minLength,43);assert.equal(query[3].schema.maxLength,43);
});
test('V075 task candidates retain minimal candidate envelope and safe no-store errors',()=>{
 const op=api.paths[path]?.get;assert.ok(op,'task candidates GET missing');
 assert.equal(op.responses['200'].content['application/json'].schema.$ref,'#/components/schemas/ReferenceCandidatesEnvelope');
 for(const status of ['200','400','401','403','404','409','503'])assert.equal(op.responses[status].headers['Cache-Control'].schema.const,'no-store');
 assert.equal(op.responses['409'].content['application/json'].schema.$ref,'#/components/schemas/WorkflowTaskErrorEnvelope');
});
