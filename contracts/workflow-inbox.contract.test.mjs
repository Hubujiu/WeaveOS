import test from 'node:test';
import {createHash} from 'node:crypto';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
const api=JSON.parse(readFileSync(new URL('./openapi/openapi.json',import.meta.url)));
const schemas=api.components.schemas;
test('personal inbox is exact guarded read-only POST without actor/resource input',()=>{
 const path=api.paths['/api/v1/workflow-tasks/search'];assert.ok(path,'personal inbox route missing');assert.deepEqual(Object.keys(path),['post']);
 const op=path.post;assert.equal(op['x-max-body-bytes'],4096);assert.deepEqual(op.security,[{WebSession:[]}]);
 assert.ok(op.parameters.some(p=>p.$ref==='#/components/parameters/ExpectedActor'));assert.ok(op.parameters.some(p=>p.$ref==='#/components/parameters/CsrfToken'));
 assert.equal(op.requestBody.content['application/json'].schema.$ref,'#/components/schemas/WorkflowInboxSearch');
 assert.equal(op.responses['200'].content['application/json'].schema.$ref,'#/components/schemas/WorkflowInboxResultEnvelope');
 const req=schemas.WorkflowInboxSearch;assert.equal(req.additionalProperties,false);assert.deepEqual(req.required,['page']);assert.deepEqual(Object.keys(req.properties).sort(),['page','pageSize','queryVersion']);
 assert.equal(req.properties.pageSize.minimum,1);assert.equal(req.properties.pageSize.maximum,100);assert.equal(req.properties.pageSize.default,20);
});
test('personal inbox returns only current task routing metadata and bounded pagination',()=>{
 const item=schemas.WorkflowInboxItem;assert.ok(item,'personal inbox summary missing');assert.equal(item.additionalProperties,false);
 const keys=['id','appId','viewId','recordId','instanceId','flowId','nodeId','flowName','createdAt','definitionVersion','activationEpoch','sequence'].sort();assert.deepEqual(item.required.toSorted(),keys);assert.deepEqual(Object.keys(item.properties).sort(),keys);
 for(const k of ['definitionVersion','activationEpoch','sequence']){assert.equal(item.properties[k].maximum,9007199254740991);assert.equal(item.properties[k].minimum,k==='sequence'?0:1);}
 const res=schemas.WorkflowInboxResult;assert.equal(res.additionalProperties,false);assert.deepEqual(res.required.toSorted(),['items','page','pageSize','queryVersion','total']);assert.equal(res.properties.items.maxItems,100);
});

test('personal inbox performance-only migration is registered with exact bytes',()=>{
 const path='migrations/00026_workflow_personal_inbox_index.sql';
 const manifest=JSON.parse(readFileSync(new URL('../infra/server/deploy/compatibility.json',import.meta.url)));
 const entry=manifest.migrations.filter(e=>e.path===path);assert.equal(entry.length,1,'personal-order migration not registered');
 const raw=readFileSync(new URL('../db/'+path,import.meta.url),'utf8').replaceAll('\r\n','\n');
 assert.equal(entry[0].sha256,createHash('sha256').update(raw).digest('hex'));
});
