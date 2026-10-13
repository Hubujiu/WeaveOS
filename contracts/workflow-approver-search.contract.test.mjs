import Ajv2020 from 'ajv/dist/2020.js';import test from 'node:test';import assert from 'node:assert/strict';import {readFileSync} from 'node:fs';
// V075 P16: configuration eligibility, not a record-filter member directory.
const api=JSON.parse(readFileSync(new URL('./openapi/openapi.json',import.meta.url))),s=api.components.schemas;
test('V075 eligible approver discovery is a manager CSRF protected read POST',()=>{const p=api.paths['/api/v1/applications/{appId}/forms/{viewId}/workflows/approvers/search'];assert.ok(p);assert.deepEqual(Object.keys(p),['post']);assert.deepEqual(p.post.security,[{WebSession:[]}]);for(const name of ['ExpectedActor','CsrfToken'])assert.ok(p.post.parameters.some(x=>x.$ref==='#/components/parameters/'+name));assert.equal(p.post.requestBody.content['application/json'].schema.$ref,'#/components/schemas/WorkflowApproverSearch');assert.equal(p.post.responses['200'].content['application/json'].schema.$ref,'#/components/schemas/WorkflowApproverResultEnvelope');});
test('V075 approver exposes only active identity, account label and current department names',()=>{const q=s.WorkflowApproverItem;assert.ok(q);assert.equal(q.additionalProperties,false);assert.deepEqual(q.required,['id','label','status','departmentNames']);assert.deepEqual(Object.keys(q.properties),q.required);assert.equal(q.properties.id.format,'uuid');assert.equal(q.properties.label.maxLength,254);assert.deepEqual(q.properties.status.enum,['active']);});
test('V075 approver result binds current schema and policy without granting record rights',()=>{const q=s.WorkflowApproverResult;assert.ok(q);assert.equal(q.additionalProperties,false);assert.deepEqual(q.required,['items','total','page','pageSize','queryVersion','schemaVersion','policyRevision']);assert.deepEqual(Object.keys(q.properties),q.required);assert.equal(q.properties.items.maxItems,100);assert.equal(q.properties.items.items.$ref,'#/components/schemas/WorkflowApproverItem');for(const key of ['schemaVersion','policyRevision']){assert.equal(q.properties[key].type,'integer');assert.equal(q.properties[key].minimum,1);assert.equal(q.properties[key].maximum,9007199254740991);}});

// Independent extension: V075 ADR P16 account/current-department prefix search, 2026-10-13 12:52 UTC.
test('P16 search query is optional bounded literal text and never changes manager input',()=>{
 const q=s.WorkflowApproverSearch;assert.ok(q,'dedicated searchable approver input is missing');assert.equal(q.additionalProperties,false);
 assert.deepEqual(q.required,['page','pageSize']);assert.deepEqual(Object.keys(q.properties).sort(),['page','pageSize','q','queryVersion']);
 const validate=new Ajv2020({strict:false,validateFormats:false}).compile(q);
 for(const value of ['', '财务', '%_', '🦊'.repeat(100)])assert.equal(validate({page:1,pageSize:20,q:value}),true);
 assert.equal(validate({page:1,pageSize:20}),true);
 for(const value of [null,3,{},[], '🦊'.repeat(101), 'nul\u0000query'])assert.equal(validate({page:1,pageSize:20,q:value}),false,JSON.stringify(value));
 assert.equal(new Ajv2020({strict:false}).compile(s.WorkflowManagerSearch)({page:1,pageSize:20,q:'not allowed'}),false);
});

test('P16 current department names are required bounded unique strings, not directory identities',()=>{
 const validate=new Ajv2020({strict:false,validateFormats:false}).compile(s.WorkflowApproverItem),base={id:'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',label:'审批人',status:'active'};
 for(const departmentNames of [[],['财务部'],['产品部','财务部'],['🦊'.repeat(100)]])assert.equal(validate({...base,departmentNames}),true);
 for(const departmentNames of [null,'财务部',[null],[''],['nul\0name'],['🦊'.repeat(101)],['财务部','财务部'],[{id:base.id,label:'财务部'}]])assert.equal(validate({...base,departmentNames}),false,JSON.stringify(departmentNames));
 assert.equal(validate(base),false);
});
