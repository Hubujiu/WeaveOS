import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
const api=JSON.parse(readFileSync(new URL('./openapi/openapi.json',import.meta.url)));
const schemas=api.components.schemas;
const path='/api/v1/applications/{appId}/forms/{viewId}/records/{recordId}/workflow-start-options/search';
test('manual option discovery is a guarded read-only POST with bounded closed paging',()=>{
 const op=api.paths[path]?.post;assert.ok(op,'safe manual options route missing');
 assert.deepEqual(Object.keys(api.paths[path]),['post']);assert.deepEqual(op.security,[{WebSession:[]}]);
 for(const name of ['ExpectedActor','CsrfToken'])assert.ok(op.parameters.some(p=>p.$ref===`#/components/parameters/${name}`));
 assert.equal(op['x-max-body-bytes'],4096);
 assert.equal(op.requestBody.content['application/json'].schema.$ref,'#/components/schemas/WorkflowManualOptionsSearch');
 assert.equal(op.responses['200'].content['application/json'].schema.$ref,'#/components/schemas/WorkflowManualOptionsEnvelope');
 const request=schemas.WorkflowManualOptionsSearch;assert.equal(request.additionalProperties,false);assert.deepEqual(request.required,['page']);
 assert.deepEqual(Object.keys(request.properties).sort(),['page','pageSize','queryVersion']);
 assert.equal(request.properties.pageSize.default,20);assert.equal(request.properties.pageSize.maximum,100);
});
test('manual options reveal only six safe identity and CAS fields',()=>{
 const item=schemas.WorkflowManualOption;assert.ok(item,'safe DTO missing');assert.equal(item.additionalProperties,false);
 const keys=['flowId','name','workflowRevision','definitionVersion','schemaVersion','recordVersion'];
 assert.deepEqual(item.required,keys);assert.deepEqual(Object.keys(item.properties),keys);
 for(const key of keys.slice(2)){assert.equal(item.properties[key].minimum,1);assert.equal(item.properties[key].maximum,9007199254740991);}
 const page=schemas.WorkflowManualOptionsResult;assert.equal(page.additionalProperties,false);assert.deepEqual(page.required,['items','total','page','pageSize','queryVersion']);
 assert.equal(page.properties.items.items.$ref,'#/components/schemas/WorkflowManualOption');
 assert.equal(page.properties.items.maxItems,100);
});
