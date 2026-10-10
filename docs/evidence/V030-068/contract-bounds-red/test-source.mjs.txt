import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
const api=JSON.parse(readFileSync(new URL('../../contracts/openapi/openapi.json',import.meta.url)));
const base='/api/v1/applications/{appId}/forms/{viewId}/workflows/{flowId}';
// Independent source: V030-068 PRD D01/D05 and Data seven-field status contract.
test('deletion POST and original status GET retain current identity and safe statuses',()=>{
 const post=api.paths[base+'/delete']?.post,get=api.paths[base+'/deletions/{operationId}']?.get;
 assert.ok(post,'durable deletion POST missing');assert.ok(get,'original operation status GET missing');
 assert.deepEqual(post.security,[{WebSession:[]}]);assert.deepEqual(get.security,post.security);
 for(const op of [post,get]){
  assert.ok(op.parameters.some(x=>x.$ref==='#/components/parameters/ExpectedActor'));
  for(const status of ['200','400','401','403','404','409','503'])assert.ok(op.responses[status]);
  assert.equal(op.responses['200'].content['application/json'].schema.$ref,'#/components/schemas/WorkflowDeletionEnvelope');
 }
 assert.ok(post.parameters.some(x=>x.$ref==='#/components/parameters/CsrfToken'));
 assert.equal(post.responses['202'].content['application/json'].schema.$ref,'#/components/schemas/WorkflowDeletionEnvelope');
 assert.ok(post.responses['202'].headers.Location);
 assert.equal(get.responses['202'],undefined);
 assert.equal(post.requestBody.content['application/json'].schema.$ref,'#/components/schemas/WorkflowDeletionRequest');
});
test('deletion status is exact seven-field DTO without executable or internal state',()=>{
 const s=api.components.schemas.WorkflowDeletionResult;assert.ok(s,'deletion status schema missing');
 const fields=['operationId','flowId','status','reason','deletedVersions','deletedAt','completedAt'].sort();
 assert.deepEqual(Object.keys(s.properties).sort(),fields);assert.deepEqual([...s.required].sort(),fields);assert.equal(s.additionalProperties,false);
 assert.deepEqual(s.properties.status.enum,['pending','unknown','deleted']);
 assert.deepEqual(s.properties.deletedVersions.type,['integer','null']);assert.equal(s.properties.deletedVersions.minimum,0);assert.equal(s.properties.deletedVersions.maximum,9007199254740991);
 for(const f of ['deletedAt','completedAt']){assert.deepEqual(s.properties[f].type,['string','null']);assert.equal(s.properties[f].format,'date-time');assert.equal(s.properties[f].pattern,'Z$');}
 const env=api.components.schemas.WorkflowDeletionEnvelope;assert.deepEqual(env.required,['code','message','data','meta']);assert.equal(env.additionalProperties,false);assert.equal(env.properties.data.$ref,'#/components/schemas/WorkflowDeletionResult');
});

test('deletion request constrains nonzero canonical identity and safe positive revision',()=>{
 const s=api.components.schemas.WorkflowDeletionRequest;assert.ok(s,'dedicated bounded deletion request missing');
 assert.equal(s.additionalProperties,false);assert.deepEqual([...s.required].sort(),['expectedRevision','operationId']);
 assert.deepEqual(Object.keys(s.properties).sort(),['expectedRevision','operationId']);
 assert.equal(s.properties.expectedRevision.minimum,1);assert.equal(s.properties.expectedRevision.maximum,9007199254740991);
 assert.equal(s.properties.operationId.not.const,'00000000-0000-0000-0000-000000000000');
 assert.equal(s.properties.operationId.pattern,'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$');
 for(const op of [api.paths[base+'/delete'].post,api.paths[base+'/deletions/{operationId}'].get])for(const p of op.parameters.filter(p=>p.in==='path'))assert.equal(p.schema.not.const,'00000000-0000-0000-0000-000000000000');
});
