import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {validateInstalledPersonnelRoles} from '../../infra/server/deploy/personnel-upgrade.mjs';

// Independent oracle: frozen B5a.1–10, PRD/ADR009 readback 2026-10-03 02:21 UTC.
const api=JSON.parse(readFileSync('contracts/openapi/openapi.json','utf8'));
const methods={
 '/api/v1/applications':['get','post'],
 '/api/v1/applications/{appId}':['get'],
 '/api/v1/applications/{appId}/access':['get'],
 '/api/v1/applications/{appId}/permission-groups':['get','post'],
 '/api/v1/applications/{appId}/permission-groups/{groupId}':['put'],
 '/api/v1/applications/{appId}/permission-groups/{groupId}/members':['get','put'],
 '/api/v1/applications/{appId}/permission-groups/{groupId}/grants':['get','put'],
 '/api/v1/application-operations/{operationId}':['get'],
};
function schema(value){return value.$ref?api.components.schemas[value.$ref.split('/').at(-1)]:value;}
test('B5a frozen HTTP routes, method sets and live Web Session are explicit',()=>{
 assert.equal(api.openapi,'3.2.1');
 for(const [path,expected] of Object.entries(methods)){
  const route=api.paths[path];assert.ok(route,'missing '+path);
  assert.deepEqual(Object.keys(route).filter(k=>['get','post','put','delete','patch'].includes(k)).sort(),expected.sort());
  for(const method of expected){const op=route[method];assert.deepEqual(op.security,[{WebSession:[]}]);assert.ok(op.responses['401']);assert.ok(op.responses['403']);
   if(['post','put'].includes(method)){assert.ok(op.parameters.some(p=>p.$ref==='#/components/parameters/CsrfToken'));const s=schema(op.requestBody.content['application/json'].schema);assert.equal(s.additionalProperties,false);assert.ok(s.required.includes('operationId'));assert.equal(s.properties.operationId.format,'uuid');if(path!=='/api/v1/applications')assert.ok(s.required.includes('expectedPolicyRevision'));assert.ok(op.responses['409']);assert.ok(op.responses['503']);}
  }
 }
});
test('B5a DTOs use exact closed fields and finite menu grants',()=>{
 const fields={ApplicationCreateRequest:['name','operationId'],ApplicationGroupCreateRequest:['name','operationId','expectedPolicyRevision'],ApplicationGroupUpdateRequest:['name','enabled','operationId','expectedPolicyRevision'],ApplicationMembersReplaceRequest:['memberIds','operationId','expectedPolicyRevision'],ApplicationGrantsReplaceRequest:['grants','operationId','expectedPolicyRevision']};
 for(const [name,expected] of Object.entries(fields)){const s=api.components.schemas[name];assert.ok(s,name);assert.deepEqual([...s.required].sort(),[...expected].sort());assert.deepEqual(Object.keys(s.properties).sort(),[...expected].sort());assert.equal(s.additionalProperties,false)}
 const grant=api.components.schemas.ApplicationMenuGrant;assert.ok(grant);assert.deepEqual([...grant.required].sort(),['resourceKind','resourceId','action','rowScope','fields'].sort());assert.equal(grant.additionalProperties,false);assert.equal(grant.properties.resourceKind.const,'application');assert.equal(grant.properties.action.const,'menu.enter');assert.equal(grant.properties.rowScope.const,'all');assert.equal(grant.properties.fields.maxItems,0);
});
test('B5a six public error codes retain frozen HTTP status',()=>{
 const codes=JSON.parse(readFileSync('contracts/errors/codes.json','utf8'));
 for(const [code,status] of Object.entries({APPLICATION_NOT_FOUND:404,APPLICATION_FORBIDDEN:403,APPLICATION_POLICY_CONFLICT:409,APPLICATION_OPERATION_CONFLICT:409,APPLICATION_OPERATION_UNCONFIRMED:503,APPLICATION_RESOURCE_INVALID:400})){assert.equal(codes[code]?.httpStatus,status,code);assert.equal(codes[code].public,true)}
});
test('B5a new migrations have exact reviewed compatibility hashes and runtime role pin',()=>{
 const manifest=JSON.parse(readFileSync('infra/server/deploy/compatibility.json','utf8'));assert.equal(manifest.backwardCompatible,false,'V068 paired deletion upgrade is not automatically promotable; old migration digests remain unchanged');
 for(const path of ['archive-migrations/00003_apps_audit.sql','migrations/00006_apps_policy.sql']){const expected=createHash('sha256').update(readFileSync('db/'+path,'utf8').replace(/\r\n/g,'\n')).digest('hex');assert.equal(manifest.migrations.find(m=>m.path===path)?.sha256,expected,path)}
 assert.doesNotThrow(()=>validateInstalledPersonnelRoles(readFileSync('infra/runtime/roles.sql','utf8')));
});
