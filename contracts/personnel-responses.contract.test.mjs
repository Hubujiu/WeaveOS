import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {assertResponseSchema} from '../tests/acceptance/response-schema.mjs';
const id='00000000-0000-4000-8000-000000000001';
const requestId='1'.repeat(32);
const template={id,name:'空模板',description:'',version:1,permissionCodes:[],templateIds:[],affectedMembers:0,affectedIdentities:0};
function response(data){return new Response(JSON.stringify({code:'OK',message:'成功',data,meta:{requestId}}),{status:200,headers:{'Content-Type':'application/json','X-Request-ID':requestId,'Cache-Control':'no-store'}});}
test('Q25 definition response accepts optional explicit empty templateIds and integer version',async()=>{
 await assertResponseSchema(response(template),'/api/v1/personnel/templates/'+id,'GET');
});
test('Q25 response validator resolves pagination query without ignoring declared constraints',async()=>{
 await assertResponseSchema(response({items:[template],total:1,page:1,pageSize:20}),'/api/v1/personnel/templates?page=1&pageSize=20','GET');
});
test('Q25 invalid definition response integer, UUID and length fail independently',async()=>{
 for(const invalid of [{...template,version:1.5},{...template,id:'invalid'},{...template,name:'x'.repeat(101)},{...template,version:0}])await assert.rejects(()=>assertResponseSchema(response(invalid),'/api/v1/personnel/templates/'+id,'GET'));
});
test('Q25 template DTO optional relation array is registered in the executable contract',()=>{
 const spec=JSON.parse(readFileSync('contracts/openapi/openapi.json','utf8'));
 assert.equal(spec.components.schemas.PersonnelTemplate.properties.templateIds?.type,'array');
});
