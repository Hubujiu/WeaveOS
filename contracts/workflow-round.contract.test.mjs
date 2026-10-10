import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
const api=JSON.parse(readFileSync(new URL('./openapi/openapi.json',import.meta.url)));
const base='/api/v1/applications/{appId}/forms/{viewId}/records/{recordId}/workflows/{instanceId}';
test('round actions expose current preview and guarded separate writes',()=>{
 assert.deepEqual(Object.keys(api.paths[base+'/round-actions']??{}).sort(),['get','post']);
 for(const suffix of ['/round-actions','/rework']){
  const op=api.paths[base+suffix]?.post;assert.ok(op);
  for(const name of ['ExpectedActor','CsrfToken'])assert.ok(op.parameters.some(p=>p.$ref==='#/components/parameters/'+name));
  assert.deepEqual(op.security,[{WebSession:[]}]);
 }
 assert.equal(api.paths[base+'/round-actions'].post.responses['202'].content['application/json'].schema.$ref,'#/components/schemas/WorkflowRoundStartResultEnvelope');
});
test('round acceptance and preview schemas are closed minimum facts',()=>{
 const s=api.components.schemas;
 const r=s.WorkflowRoundStart;assert.ok(r);assert.equal(r.additionalProperties,false);
 assert.deepEqual(r.required,['operationId','kind','expectedWorkflowRevision','expectedSchemaVersion','expectedRecordVersion']);
 assert.deepEqual(r.properties.kind.enum,['resubmit','review']);
 const v=s.WorkflowRoundStartResult;assert.ok(v);assert.equal(v.additionalProperties,false);
 assert.deepEqual(v.required,['operationId','flowId','previousInstanceId','instanceId','roundKind','roundNumber','status']);
 assert.deepEqual(v.properties.status.enum,['accepted']);assert.equal(v.properties.values,undefined);
 const p=s.WorkflowRoundPreview;assert.ok(p);assert.equal(p.additionalProperties,false);
 assert.deepEqual(p.required,['instanceId','flowId','roundNumber','latestInstanceId','state','definitionVersion','workflowRevision','schemaVersion','recordVersion','canRework','canResubmit','canReview','editableFieldIds']);
 assert.equal(p.properties.graph,undefined);assert.equal(p.properties.approvers,undefined);
});
