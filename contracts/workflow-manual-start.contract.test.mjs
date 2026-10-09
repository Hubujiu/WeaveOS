import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
const api=JSON.parse(readFileSync(new URL('./openapi/openapi.json',import.meta.url)));
const schemas=api.components.schemas;
const path='/api/v1/applications/{appId}/forms/{viewId}/records/{recordId}/workflow-starts';
test('manual start has only a guarded POST returning durable acceptance',()=>{
 const op=api.paths[path]?.post;assert.ok(op,'ordinary manual start POST missing');
 assert.deepEqual(Object.keys(api.paths[path]),['post']);
 assert.equal(op['x-max-body-bytes'],4096);
 assert.ok(op.parameters.some(p=>p.$ref==='#/components/parameters/ExpectedActor'));
 assert.ok(op.parameters.some(p=>p.$ref==='#/components/parameters/CsrfToken'));
 assert.deepEqual(op.security,[{WebSession:[]}]);
 assert.equal(op.requestBody.content['application/json'].schema.$ref,'#/components/schemas/WorkflowManualStart');
 assert.equal(op.responses['202'].content['application/json'].schema.$ref,'#/components/schemas/WorkflowManualStartResultEnvelope');
});
test('manual request and receipt exclude trusted facts and premature engine success',()=>{
 const req=schemas.WorkflowManualStart;assert.ok(req);assert.equal(req.additionalProperties,false);
 assert.deepEqual(req.required,['operationId','flowId','expectedWorkflowRevision','expectedSchemaVersion','expectedRecordVersion']);
 for(const key of req.required.slice(2)){assert.equal(req.properties[key].minimum,1);assert.equal(req.properties[key].maximum,9007199254740991);}
 const res=schemas.WorkflowManualStartResult;assert.ok(res);assert.equal(res.additionalProperties,false);
 assert.deepEqual(res.required,['operationId','flowId','instanceId','status','ignored']);
 assert.deepEqual(res.properties.status.enum,['accepted','ignored']);
 assert.equal(res.properties.commandId,undefined);assert.equal(res.properties.values,undefined);
 assert.equal(res.properties.ignored.type,'boolean');
});
