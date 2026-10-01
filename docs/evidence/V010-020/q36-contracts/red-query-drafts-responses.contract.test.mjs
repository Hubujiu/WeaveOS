import test from 'node:test';
import assert from 'node:assert/strict';
import {assertResponseSchema} from '../tests/acceptance/response-schema.mjs';
const id='00000000-0000-4000-8000-000000000001';
const requestId='1'.repeat(32);
const summary={id,kind:'identity',targetId:null,baseVersion:null,version:1,createdAt:'2026-10-01T00:00:00Z',updatedAt:'2026-10-01T00:00:00Z'};
const payload={name:'草稿',description:'',templateIds:[],permissionCodes:[]};
function response(data){return new Response(JSON.stringify({code:'OK',message:'成功',data,meta:{requestId}}),{status:200,headers:{'Content-Type':'application/json','X-Request-ID':requestId,'Cache-Control':'no-store'}});}
test('Q36 response oracle enforces draft capacity and payload bounds instead of ignoring extensions',async()=>{
 await assertResponseSchema(response({items:Array.from({length:20},(_,i)=>({...summary,id:`00000000-0000-4000-8000-${String(i).padStart(12,'0')}`}))}),'/api/v1/personnel/drafts','GET');
 await assert.rejects(()=>assertResponseSchema(response({items:Array(21).fill(summary)}),'/api/v1/personnel/drafts','GET'));
 await assertResponseSchema(response({...summary,payload}),'/api/v1/personnel/drafts/'+id,'GET');
 // Independently over 64KiB from 500 distinct, <=160-character permission codes.
 const large={...payload,permissionCodes:Array.from({length:500},(_,i)=>String(i).padStart(4,'0')+'x'.repeat(140))};
 await assert.rejects(()=>assertResponseSchema(response({...summary,payload:large}),'/api/v1/personnel/drafts/'+id,'GET'));
 await assert.rejects(()=>assertResponseSchema(response({...summary,payload:{...payload,password:'synthetic-forbidden-field'}}),'/api/v1/personnel/drafts/'+id,'GET'));
});
