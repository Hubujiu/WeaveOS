import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
const api=JSON.parse(readFileSync(new URL('./openapi/openapi.json',import.meta.url))),s=api.components.schemas;
test('personal approved history has exact authenticated read-only paging admission',()=>{
 const p=api.paths['/api/v1/workflow-approvals/search'];assert.ok(p,'personal approved route missing');assert.deepEqual(Object.keys(p),['post']);const op=p.post;
 assert.equal(op['x-max-body-bytes'],4096);assert.deepEqual(op.security,[{WebSession:[]}]);for(const n of ['ExpectedActor','CsrfToken'])assert.ok(op.parameters.some(x=>x.$ref==='#/components/parameters/'+n));
 assert.equal(op.requestBody.content['application/json'].schema.$ref,'#/components/schemas/WorkflowInboxSearch');assert.equal(op.responses['200'].content['application/json'].schema.$ref,'#/components/schemas/WorkflowApprovalsResultEnvelope');
});
test('personal approved history contains only a current resource locator and independent event',()=>{
 const item=s.WorkflowApprovalsItem;assert.ok(item,'personal approved item missing');assert.equal(item.additionalProperties,false);assert.deepEqual(item.required.toSorted(),['appId','event','recordId','viewId']);assert.deepEqual(Object.keys(item.properties).sort(),['appId','event','recordId','viewId']);
 assert.equal(item.properties.event.$ref,'#/components/schemas/WorkflowApprovedEvent');const event=s.WorkflowApprovedEvent;assert.equal(event.allOf[0].$ref,'#/components/schemas/WorkflowEventSummary');assert.deepEqual(event.allOf[1].properties.action.enum,['agree','reject','return']);assert.equal(event.allOf[1].properties.outcome.const,'success');
});
test('personal approved page has closed complete counts and no historical business values',()=>{
 const r=s.WorkflowApprovalsResult;assert.ok(r,'personal approved result missing');assert.equal(r.additionalProperties,false);assert.deepEqual(r.required.toSorted(),['items','page','pageSize','queryVersion','total']);assert.deepEqual(Object.keys(r.properties).sort(),['items','page','pageSize','queryVersion','total']);assert.equal(r.properties.items.maxItems,100);assert.equal(r.properties.items.items.$ref,'#/components/schemas/WorkflowApprovalsItem');assert.equal(r.properties.total.maximum,Number.MAX_SAFE_INTEGER);
});
