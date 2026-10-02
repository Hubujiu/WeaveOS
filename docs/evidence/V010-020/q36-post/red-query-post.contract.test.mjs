import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
// Oracle: user-approved POST amendment, PLAN §9. Schema assertions only.
const api=JSON.parse(readFileSync('contracts/openapi/openapi.json'));
test('Q36 approved POST searches carry object filters with route-local raw-body bounds and CSRF',()=>{
 for(const [view,prefix] of [['members','Member'],['events','Event']]){
  const op=api.paths[`/api/v1/personnel/${view}/search`]?.post;
  assert.ok(op,view+' POST search missing');
  assert.deepEqual(op.security,[{WebSession:[]}]);
  assert.ok(op.parameters.some(p=>p.$ref?.endsWith('/CsrfToken')));
  const body=op.requestBody;
  assert.equal(body['x-max-raw-bytes'],65536);
  const schema=api.components.schemas[prefix+'SearchInput'];
  assert.equal(schema.properties.filter.$ref,`#/components/schemas/${prefix}FilterGroup`);
  assert.equal(api.components.schemas[prefix+'FilterGroup']['x-max-canonical-bytes'],16384);
  for(const key of ['page','pageSize','search','queryVersion'])assert.ok(schema.properties[key]);
  for(const status of ['200','400','401','403','409','415','503'])assert.ok(op.responses[status]);
  assert.match(op.description,/no business writes/i);
 }
});
test('Q36 legacy GET has no complex filter/sort; draft raw wrapper budget exceeds canonical payload',()=>{
 for(const view of ['members','events']){
  const op=api.paths[`/api/v1/personnel/${view}`].get;
  assert.equal(op.deprecated,true);
  assert.equal(op.parameters.some(p=>['filter','sortBy','sortDirection'].includes(p.name)),false);
  assert.match(op.description,/queryVersion.*write protection/i);
 }
 for(const [path,method] of [['/api/v1/personnel/drafts','post'],['/api/v1/personnel/drafts/{draftId}','put']])
  assert.equal(api.paths[path][method].requestBody['x-max-raw-bytes'],1048576);
});
