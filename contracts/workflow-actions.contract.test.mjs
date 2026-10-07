// Root-owned public task-action contract; runtime behavior is covered by Go HTTPS tests.
import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
const api=JSON.parse(readFileSync(new URL('./openapi/openapi.json',import.meta.url)));
const codes=JSON.parse(readFileSync(new URL('./errors/codes.json',import.meta.url)));
const root='/api/v1/applications/{appId}/forms/{viewId}/records/{recordId}/workflow-instances/{instanceId}/tasks/{taskId}';
const ref=n=>'#/components/schemas/'+n;
test('Root V037: exact preview/action/status routes use existing Session and CSRF',()=>{
 assert.equal(api.openapi,'3.2.1');
 for(const [path,method]of [[root,'get'],[root+'/actions','post'],['/api/v1/application-workflow-operations/{operationId}','get']]){
  const op=api.paths[path]?.[method];assert.ok(op,path);assert.deepEqual(op.security,[{WebSession:[]}]);
  assert.ok(op.parameters.some(p=>p.$ref==='#/components/parameters/ExpectedActor'));
  for(const status of ['200','400','401','403','404','409','503']){assert.ok(op.responses[status]);assert.equal(op.responses[status].headers['Cache-Control'].schema.const,'no-store');}
 }
 const post=api.paths[root+'/actions'].post;assert.ok(post.parameters.some(p=>p.$ref==='#/components/parameters/CsrfToken'));
 assert.equal(post.requestBody['x-max-body-bytes'],4096);assert.equal(post.requestBody.content['application/json'].schema.$ref,ref('WorkflowTaskAction'));
 assert.equal(post.responses['202'].content['application/json'].schema.$ref,ref('WorkflowTaskPendingEnvelope'));assert.ok(post.responses['202'].headers.Location);
});
test('Root V037: exact closed action body and separate pending/final result shapes',()=>{
 const s=api.components.schemas;assert.deepEqual(s.WorkflowTaskAction.required,['operationId','action','basisToken']);assert.equal(s.WorkflowTaskAction.additionalProperties,false);assert.deepEqual(s.WorkflowTaskAction.properties.action.enum,['agree','reject']);
 assert.deepEqual(s.WorkflowTaskPending.required,['operationId','commandId','instanceId','status']);assert.equal(s.WorkflowTaskPending.additionalProperties,false);assert.equal(s.WorkflowTaskPending.properties.status.const,'pending');
 assert.equal(s.WorkflowTaskSuccess.properties.status.const,'success');assert.equal(s.WorkflowTaskNoEffect.properties.status.const,'no_effect');
 assert.deepEqual(s.WorkflowTaskOperation.oneOf,[{$ref:ref('WorkflowTaskPending')},{$ref:ref('WorkflowTaskSuccess')},{$ref:ref('WorkflowTaskNoEffect')}]);
 assert.deepEqual(s.ApplicationOperation.properties.httpStatus.enum,[200,201,202,204]);assert.ok(s.ApplicationOperation.properties.result.oneOf.some(x=>x.$ref===ref('WorkflowTaskPending')));
 assert.equal(s.WorkflowTaskPreview.properties.editableFieldIds.type,'array');assert.equal(s.WorkflowTaskPreview.properties.editableFieldIds.uniqueItems,true);assert.equal(s.WorkflowTaskPreview.properties.editableFieldIds.maxItems,200);
 assert.deepEqual(s.WorkflowTaskPreview.required,['basisToken','task','record','fields','editableFieldIds']);assert.equal(s.WorkflowTaskPreview.properties.fields.items.$ref,ref('Field'));assert.equal(s.WorkflowTaskPreview.properties.record.$ref,ref('BusinessRecord'));
 for(const name of ['WorkflowTaskPending','WorkflowTaskSuccess','WorkflowTaskNoEffect'])for(const forbidden of ['evidenceHash','values','fields','payload','routes','actorId'])assert.equal(s[name].properties[forbidden],undefined);
});
test('Root V037: stale task/basis and oversized real evidence have public safe conflicts',()=>{
 for(const code of ['WORKFLOW_TASK_CHANGED','WORKFLOW_BASIS_CHANGED','WORKFLOW_BASIS_EXPIRED','WORKFLOW_EVIDENCE_TOO_LARGE']){
  assert.deepEqual(codes[code],{httpStatus:409,grpcStatus:'FAILED_PRECONDITION',public:true});assert.ok(api.components.schemas.WorkflowTaskErrorEnvelope.allOf[1].properties.code.enum.includes(code));
 }
});
