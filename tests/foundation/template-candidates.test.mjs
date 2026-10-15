import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {stripTypeScriptTypes} from 'node:module';
// V075 P21 / V070 current target candidates; independently specified closed wire.
const source=await readFile(new URL('../../apps/web/src/applications/templates/candidates.ts',import.meta.url),'utf8');
const {templateCandidatePage}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(source,{mode:'transform'})).toString('base64'));
const id='eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee',parent='dddddddd-dddd-4ddd-8ddd-dddddddddddd';
const user={id,label:'当前用户',status:'active'},department={...user,label:'当前部门',parentId:parent};
const page=(items=[],nextPageToken=null)=>({data:{items},meta:{requestId:'request-1',pagination:{nextPageToken,hasMore:nextPageToken!==null}}});
test('template candidates consume only current user or department minimal pages with opaque continuation',()=>{
 assert.deepEqual(templateCandidatePage(page([user]),'user'),{items:[user],nextPageToken:null});
 assert.deepEqual(templateCandidatePage(page([department],'A'.repeat(43)),'department'),{items:[department],nextPageToken:'A'.repeat(43)});
 assert.deepEqual(templateCandidatePage(page([{...department,parentId:null}]),'department'),{items:[{...department,parentId:null}],nextPageToken:null});
 assert.deepEqual(templateCandidatePage(page(),'user'),{items:[],nextPageToken:null});
});
test('template candidates reject malformed data, extra identity information and cross-kind projections',()=>{
 for(const item of [{...user,email:'private@example.test'},{...user,status:'disabled'},{...user,status:'deleted'},{...user,id:'bad'},{...user,id:id.toUpperCase()},{...user,id:'00000000-0000-0000-0000-000000000000'},{...user,label:null},department])assert.equal(templateCandidatePage(page([item]),'user'),null);
 for(const item of [user,{...department,parentId:'bad'},{...department,extra:true}])assert.equal(templateCandidatePage(page([item]),'department'),null);
 for(const value of [null,[],{data:null,meta:null},{...page(),data:{items:[],total:0}},{...page(),data:{items:null}},page([user,user]),page(Array.from({length:51},()=>user))])assert.equal(templateCandidatePage(value,'user'),null);
});
test('template candidate pagination must be present, closed, bounded and internally consistent',()=>{
 for(const pagination of [null,{}, {nextPageToken:null,hasMore:true},{nextPageToken:'A'.repeat(43),hasMore:false},{nextPageToken:'',hasMore:true},{nextPageToken:null,hasMore:false,total:0}])assert.equal(templateCandidatePage({data:{items:[]},meta:{pagination}},'user'),null);
});

test('ADR002 candidate continuation is an opaque nonempty string, not a client-decoded server format',()=>{
 for(const token of ['opaque-token','different format = + /','a'.repeat(257)])assert.deepEqual(templateCandidatePage(page([user],token),'user'),{items:[user],nextPageToken:token});
});
test('candidate metadata stays closed and requires the request identity',()=>{
 for(const meta of [{pagination:{nextPageToken:null,hasMore:false}},{requestId:12,pagination:{nextPageToken:null,hasMore:false}},{requestId:'r',pagination:{nextPageToken:null,hasMore:false},private:true}])assert.equal(templateCandidatePage({data:{items:[]},meta},'user'),null);
});
