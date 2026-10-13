import test from 'node:test';import assert from 'node:assert/strict';import {readFileSync} from 'node:fs';import Ajv2020 from 'ajv/dist/2020.js';
// Independent oracle: V075 ADR P19, 2026-10-13; V072 manager-only retained lifecycle.
const api=JSON.parse(readFileSync('contracts/openapi/openapi.json'));
const path='/api/v1/applications/{appId}/forms/{viewId}/records/deleted/search';
test('P19 deleted discovery is a bounded current-session CSRF-protected read POST',()=>{
 const op=api.paths[path]?.post;assert.ok(op,'deleted discovery public contract missing');
 assert.deepEqual(Object.keys(api.paths[path]),['post']);assert.deepEqual(op.security,[{WebSession:[]}]);
 for(const name of ['ExpectedActor','CsrfToken'])assert.ok(op.parameters.some(p=>p.$ref==='#/components/parameters/'+name));
 for(const status of ['200','400','401','403','404','409','415','503'])assert.ok(op.responses[status]);
 assert.equal(op.responses['200'].content['application/json'].schema.$ref,'#/components/schemas/DeletedRecordSearchEnvelope');
 assert.equal(op.responses['200'].headers['Cache-Control'].schema.const,'no-store');
});
test('P19 discovery input is closed and enforces safe page bounds without mutation or identity fields',()=>{
 const schema=api.components.schemas.DeletedRecordSearch;assert.ok(schema,'deleted input missing');
 assert.equal(schema.additionalProperties,false);assert.deepEqual([...schema.required].sort(),['page','pageSize']);
 assert.deepEqual(Object.keys(schema.properties).sort(),['page','pageSize','queryVersion']);
 const v=new Ajv2020({strict:false,validateFormats:false}).compile(schema);
 for(const x of [{page:1,pageSize:20},{page:3,pageSize:1,queryVersion:'opaque-context'}])assert.ok(v(x));
 for(const x of [{page:0,pageSize:20},{page:1,pageSize:101},{page:1.5,pageSize:20},{page:1,pageSize:20,queryVersion:''},{page:1,pageSize:20,queryVersion:null},{page:1,pageSize:20,operationId:'x'},{page:1,pageSize:20,actorId:'x'}])assert.equal(v(x),false,JSON.stringify(x));
});
test('P19 deleted discovery exposes only bounded retained metadata and never business values or actors',()=>{
 const result=api.components.schemas.DeletedRecordSearchResult,item=api.components.schemas.DeletedRecordItem;
 assert.ok(result&&item,'deleted output missing');
 for(const [s,keys] of [[result,['items','total','page','pageSize','queryVersion','schemaVersion']],[item,['id','recordVersion','deletedAt']]]){assert.equal(s.additionalProperties,false);assert.deepEqual(Object.keys(s.properties).sort(),keys.sort());assert.deepEqual([...s.required].sort(),keys.sort());}
 const v=new Ajv2020({strict:false,validateFormats:false}).compile(item);const good={id:'10000000-0000-4000-8000-000000000001',recordVersion:2,deletedAt:'2026-10-13T00:00:00Z'};assert.ok(v(good));
 for(const bad of [{...good,values:{}},{...good,changedBy:good.id},{...good,recordVersion:0},{...good,recordVersion:9007199254740992}])assert.equal(v(bad),false);
});
