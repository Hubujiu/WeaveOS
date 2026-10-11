import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {stripTypeScriptTypes} from 'node:module';
const source=await readFile(new URL('../../apps/web/src/applications/workflows/taskCandidateContract.ts',import.meta.url),'utf8');
const {validTaskCandidates}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(source,{mode:'transform'})).toString('base64'));
const id='77777777-7777-4777-8777-777777777777';
const member={id,label:'张三',status:'active'},department={...member,label:'财务部',parentId:null};
const end={hasMore:false,nextPageToken:null},more={hasMore:true,nextPageToken:'a'.repeat(43)};
for(const [name,items,page,kind,size]of [['member',[member],end,'member',20],['department',[department],end,'department',20],['department parent',[{...department,parentId:id}],end,'department',1],['next page',[member],more,'member',1],['empty',[],end,'member',20]])test('task candidates accepts '+name,()=>assert.equal(validTaskCandidates({items},page,kind,size),true));
for(const [name,data,page,kind,size]of [
 ['extra body',{items:[member],secret:'no'},end,'member',20],['null',null,end,'member',20],['items missing',{},end,'member',20],['items scalar',{items:member},end,'member',20],
 ['extra private item',{items:[{...member,password:'no'}]},end,'member',20],['duplicate',{items:[member,member]},end,'member',20],['disabled',{items:[{...member,status:'disabled'}]},end,'member',20],['deleted',{items:[{...member,status:'deleted'}]},end,'member',20],
 ['zero id',{items:[{...member,id:'00000000-0000-0000-0000-000000000000'}]},end,'member',20],['invalid id',{items:[{...member,id:'../other'}]},end,'member',20],['blank label',{items:[{...member,label:' '}]},end,'member',20],
 ['department missing parent',{items:[member]},end,'department',20],['member extra parent',{items:[department]},end,'member',20],['department wrong parent',{items:[{...department,parentId:3}]},end,'department',20],
 ['missing pagination',{items:[member]},null,'member',20],['contradictory page',{items:[member]},{hasMore:true,nextPageToken:null},'member',20],['unexpected cursor',{items:[member]},{hasMore:false,nextPageToken:'a'.repeat(43)},'member',20],['short cursor',{items:[member]},{hasMore:true,nextPageToken:'x'},'member',20],['extra pagination',{items:[member]},{...end,offset:1},'member',20],
 ['too many',{items:[member,{...member,id:'88888888-8888-4888-8888-888888888888'}]},end,'member',1],['bad page size',{items:[member]},end,'member',0],
])test('task candidates rejects '+name,()=>assert.equal(validTaskCandidates(data,page,kind,size),false));
