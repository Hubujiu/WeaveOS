import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
const spec=JSON.parse(readFileSync('contracts/openapi/openapi.json','utf8'));
const errors=JSON.parse(readFileSync('contracts/errors/codes.json','utf8'));
const methods={
 '/api/v1/me/access':['get'],
 '/api/v1/personnel/members':['get'],
 '/api/v1/personnel/members/{memberId}':['get'],
 '/api/v1/personnel/members/{memberId}/identities':['put'],
 '/api/v1/personnel/members/{memberId}/groups':['post'],
 '/api/v1/personnel/departments':['get','post'],
 '/api/v1/personnel/departments/{departmentId}':['put','delete'],
 '/api/v1/personnel/identities':['get','post'],
 '/api/v1/personnel/identities/{identityId}':['get','put','delete'],
 '/api/v1/personnel/templates':['get','post'],
 '/api/v1/personnel/templates/{templateId}':['get','put','delete'],
 '/api/v1/personnel/permissions':['get'],
 '/api/v1/personnel/events':['get'],
};
test('Q25 all personnel operations are typed, Cookie protected and fail closed',()=>{
 assert.equal(spec.openapi,'3.2.1');
 for(const [path,verbs] of Object.entries(methods))for(const verb of verbs){
  const op=spec.paths[path]?.[verb];assert.ok(op,path+' '+verb+' missing');
  assert.ok(op.operationId);assert.ok(op.security?.some(s=>Object.keys(s).length),'protected operation');
  for(const status of ['401','403','503'])assert.ok(op.responses[status],path+' '+status+' error absent');
  if(['post','put','delete'].includes(verb))assert.ok(op.responses['409'],path+' conflict missing');
  if(['post','put'].includes(verb)){const schema=op.requestBody?.content?.['application/json']?.schema;assert.ok(schema);assert.equal(schema.additionalProperties,false);}
  if(path.includes('{'))assert.ok(op.parameters?.some(p=>p.in==='path'&&p.required));
 }
});
test('Q25 page pagination exception is explicit, bounded, and filtered server-side',()=>{
 for(const name of ['members','identities','templates','events']){
  const op=spec.paths['/api/v1/personnel/'+name]?.get;assert.ok(op,'missing paginated '+name);
  const page=op.parameters.find(p=>p.name==='page'),size=op.parameters.find(p=>p.name==='pageSize');
  assert.equal(page.schema.default,1);assert.equal(size.schema.default,20);assert.equal(size.schema.maximum,100);
  assert.ok(op.parameters.some(p=>p.name==='search'));
 }
});
test('Q25 versions and explicit sets cannot mutate Root or directory ownership',()=>{
 const schemas=spec.components.schemas;
 for(const name of ['PersonnelIdentityInput','PersonnelTemplateInput','MemberIdentitiesInput','MemberGroupsInput','DepartmentUpdateInput']){
  const s=schemas[name];assert.ok(s,name+' missing');assert.ok(s.required.includes('version'));assert.equal(s.additionalProperties,false);assert.equal(s.properties.version.type,'integer');
  for(const forbidden of ['bootstrapAdmin','isRoot','status','scope','expiresAt'])assert.ok(!s.properties[forbidden]);
 }
 assert.ok(!spec.paths['/api/v1/personnel/permissions']?.post,'catalog owned by registration');
 assert.deepEqual(schemas.MemberGroupsInput.properties.operation.enum,['add','remove','move']);
 assert.ok(schemas.MemberIdentitiesInput.required.includes('identityIds'));
});
test('Q25 stable conflict/missing errors and invitation qualification are registered',()=>{
 assert.equal(errors.PERSONNEL_CONFLICT?.httpStatus,409);assert.equal(errors.PERSONNEL_NOT_FOUND?.httpStatus,404);
 assert.match(spec.paths['/api/v1/invitations'].post.description,/personnel\.manage/);
 assert.match(spec.paths['/api/v1/users/{userId}/password-reset'].post.description,/Bootstrap|Root/);
});
