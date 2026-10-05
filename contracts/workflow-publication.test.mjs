import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
const api=JSON.parse(fs.readFileSync(new URL('./openapi/openapi.json',import.meta.url),'utf8'));
const base='/api/v1/applications/{appId}/forms/{viewId}/workflows/{flowId}';
test('Root publication HTTP separates durable acceptance from confirmed state',()=>{
 const post=api.paths[base+'/publish']?.post,get=api.paths[base+'/publications/{operationId}']?.get;
 assert.ok(post);assert.ok(get);assert.ok(post.responses['202']);assert.ok(post.responses['200']);assert.ok(post.responses['202'].headers.Location);
 assert.deepEqual(post.security,[{WebSession:[]}]);assert.deepEqual(get.security,[{WebSession:[]}]);
 assert.ok(post.parameters.some(p=>p.$ref==='#/components/parameters/CsrfToken'));assert.ok(post.parameters.some(p=>p.$ref==='#/components/parameters/ExpectedActor'));
 assert.ok(get.parameters.some(p=>p.$ref==='#/components/parameters/ExpectedActor'));
 assert.equal(post.requestBody.content['application/json'].schema.$ref,'#/components/schemas/WorkflowPublicationRequest');
 for(const status of ['400','401','403','404','409','415','503'])assert.ok(post.responses[status]);
});
test('Root publication request is closed and never accepts engine XML or receipt',()=>{
 const s=api.components.schemas.WorkflowPublicationRequest;assert.ok(s);assert.equal(s.additionalProperties,false);
 assert.deepEqual([...s.required].sort(),['expectedRevision','expectedSchemaVersion','operationId']);
 assert.deepEqual(Object.keys(s.properties).sort(),['expectedRevision','expectedSchemaVersion','operationId']);
 for(const key of ['expectedRevision','expectedSchemaVersion']){assert.equal(s.properties[key].type,'integer');assert.equal(s.properties[key].minimum,1);assert.equal(s.properties[key].maximum,9007199254740991);}
});
test('Root publication result exposes explicit pending unknown and terminal meanings',()=>{
 const s=api.components.schemas.WorkflowPublicationResult;assert.ok(s);assert.equal(s.additionalProperties,false);
 assert.deepEqual([...s.required].sort(),['flowId','operationId','reason','status','version']);
 assert.deepEqual(Object.keys(s.properties).sort(),['flowId','operationId','reason','status','version']);
 assert.deepEqual([...s.properties.status.enum].sort(),['blocked','confirmed','pending','superseded','unknown']);
 assert.equal(s.properties.version.type,'integer');assert.equal(s.properties.version.minimum,1);
 const env=api.components.schemas.WorkflowPublicationEnvelope;assert.ok(env);assert.ok(JSON.stringify(env).includes('#/components/schemas/WorkflowPublicationResult'));
});
