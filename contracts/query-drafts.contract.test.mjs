import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
// Independent oracle: approved V010-020-PLAN §§1–5; schema-only, not runtime acceptance.
const api=JSON.parse(readFileSync('contracts/openapi/openapi.json','utf8'));
const codes=JSON.parse(readFileSync('contracts/errors/codes.json','utf8'));
const schemas=api.components.schemas;
const parameter=(name,key)=>api.paths['/api/v1/personnel/'+name].get.parameters.find(p=>p.name===key);
test('Q36 query responses retain paging and publish a session-owned opaque baseline',()=>{
 for(const name of ['PersonnelMembersPage','PersonnelEventsPage']){
  assert.ok(schemas[name].required.includes('queryVersion'),'queryVersion is absent');
  assert.ok(schemas[name].required.includes('sort'),'normalized sort is absent');
  for(const k of ['items','total','page','pageSize'])assert.ok(schemas[name].required.includes(k));
 }
 assert.ok(schemas.PersonnelEventsPage.required.includes('range'));
 assert.ok(schemas.PersonnelActivity.required.includes('display'),'safe display must be server-owned');
 assert.deepEqual(schemas.PersonnelMembersPage.properties.sort.type,'null');
 assert.deepEqual(parameter('events','sortBy').schema.enum,['occurredAt']);
 assert.equal(parameter('members','sortBy'),undefined,'members have no sortable numeric/time business column');
});
test('Q36 both query views declare grouped AND/OR, field types and global bounds',()=>{
 for(const [view,prefix] of [['members','Member'],['events','Event']]){
  assert.ok(parameter(view,'filter')?.content?.['application/json'],'GET filter is typed JSON');
  const root=schemas[prefix+'FilterGroup'];assert.ok(root,'group missing');
  assert.deepEqual(root.properties.operator.enum,['and','or']);
  assert.equal(root['x-max-group-depth'],3);assert.equal(root['x-max-leaves'],20);assert.equal(root['x-max-canonical-bytes'],16384);
  assert.equal(root.properties.children.minItems,1);
  assert.equal(schemas[prefix+'FilterGroupLevel3'].properties.children.items.$ref,'#/components/schemas/'+prefix+'FilterCondition');
 }
 const f=schemas.MemberFilterCondition.oneOf;
 const relation=f.find(v=>v.properties.field.enum.includes('identityIds'));
 assert.deepEqual(relation.properties.operator.enum,['eq','neq']);
 assert.equal(relation.properties.value.format,'uuid');
 assert.match(relation.description,/NOT EXISTS/);
 const text=f.find(v=>v.properties.field.enum.includes('account'));
 assert.deepEqual(text.properties.operator.enum,['eq','neq']);
 assert.match(schemas.EventFilterCondition.description,/NULL/);
});
test('Q36 query changes, expiry and busy are distinguishable without repurposing old errors',()=>{
 for(const c of ['COMMON_QUERY_CHANGED','COMMON_QUERY_CONTEXT_EXPIRED'])assert.equal(codes[c]?.httpStatus,409,c);
 assert.equal(codes.COMMON_SERVICE_UNAVAILABLE.httpStatus,503);
 assert.equal(schemas.QueryBusyEnvelope.properties.meta.properties.reason.const,'QUERY_BUSY');
 for(const v of ['members','events']){
  assert.ok(api.paths['/api/v1/personnel/'+v].get.responses['409']);
  assert.ok(parameter(v,'queryVersion'));
 }
});
test('Q36 object checks remain and an exact draft version can be acknowledged on submission',()=>{
 for(const n of ['MemberIdentitiesInput','MemberGroupsInput','DepartmentUpdateInput']){
  assert.ok(schemas[n].required.includes('version'));
  assert.ok(schemas[n].required.includes('queryVersion'));
  assert.equal(schemas[n].properties.draftRef.$ref,'#/components/schemas/DraftReference');
 }
 for(const n of ['PersonnelIdentityInput','PersonnelTemplateInput'])assert.ok(schemas[n].properties.draftRef);
 assert.deepEqual(schemas.DraftReference.required,['id','version']);
});
test('Q36 persistent personal drafts have explicit limits, conflict protection and no expiry',()=>{
 const p=api.paths['/api/v1/personnel/drafts'];assert.ok(p?.get&&p?.post,'draft collection missing');
 const item=api.paths['/api/v1/personnel/drafts/{draftId}'];assert.ok(item?.get&&item?.put&&item?.delete);
 assert.equal(schemas.PersonnelDraftList.properties.items.maxItems,20);
 assert.equal(schemas.DraftPayload['x-max-canonical-bytes'],65536);
 assert.ok(schemas.DraftUpdateInput.required.includes('version'));
 assert.ok(schemas.PersonnelDraft.required.includes('baseVersion'));
 assert.equal(schemas.PersonnelDraft.properties.expiresAt,undefined);
 assert.equal(schemas.DraftCreateInput.properties.ownerId,undefined);
 for(const verb of ['post'])assert.ok(p[verb].parameters.some(v=>v.$ref?.endsWith('/CsrfToken')));
 for(const verb of ['put','delete'])assert.ok(item[verb].parameters.some(v=>v.$ref?.endsWith('/CsrfToken')));
});
