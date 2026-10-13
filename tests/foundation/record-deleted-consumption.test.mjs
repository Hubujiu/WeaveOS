import test from 'node:test';import assert from 'node:assert/strict';import {readFile} from 'node:fs/promises';import {stripTypeScriptTypes} from 'node:module';
const source=await readFile(new URL('../../apps/web/src/applications/records/deletedContract.ts',import.meta.url),'utf8');
const {validDeletedRecordPage:valid}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(source,{mode:'transform'})).toString('base64'));
const id='d9d9d9d9-d9d9-4d9d-8d9d-d9d9d9d9d9d9',item={id,recordVersion:2,deletedAt:'2026-10-13T12:00:00.123456789Z'},page={items:[item],total:1,page:1,pageSize:20,queryVersion:'opaque context',schemaVersion:1};
test('P19 current retained page accepts complete first last and empty deep pages without mutating input',()=>{
 const before=JSON.stringify(page);assert.ok(valid(page,1,20));assert.equal(JSON.stringify(page),before);
 assert.ok(valid({...page,items:[],total:0},1,20));assert.ok(valid({...page,items:[],page:2},2,20));
 assert.ok(valid({...page,pageSize:1,total:2},1,1));assert.ok(valid({...page,queryVersion:'不透明 token + /'},1,20));
});
test('P19 retained page rejects closed shape identity version and real UTC calendar violations',()=>{
 for(const bad of [null,[],{...page,values:{}},{...page,items:null},{...page,schemaVersion:0},{...page,total:-1},{...page,total:Number.MAX_SAFE_INTEGER+1},{...page,queryVersion:''},{...page,queryVersion:null},{...page,queryVersion:'x'.repeat(257)}])assert.equal(valid(bad,1,20),false,JSON.stringify(bad));
 for(const patch of [{values:{}},{changedBy:id},{id:'bad'},{id:id.toUpperCase()},{id:'00000000-0000-0000-0000-000000000000'},{recordVersion:0},{recordVersion:2.2},{recordVersion:Number.MAX_SAFE_INTEGER+1},{deletedAt:'2026-02-30T00:00:00Z'},{deletedAt:'2026-02-29T00:00:00Z'},{deletedAt:'2026-10-13T24:00:00Z'},{deletedAt:'2026-10-13T00:00:00+08:00'}])assert.equal(valid({...page,items:[{...item,...patch}]},1,20),false,JSON.stringify(patch));
 for(const key of Object.keys(page)){const bad={...page};delete bad[key];assert.equal(valid(bad,1,20),false,key);}
});
test('P19 retained page binds exact requested page and complete count with unique records and safe offset',()=>{
 for(const bad of [{...page,page:2},{...page,pageSize:1},{...page,items:[]},{...page,total:0},{...page,items:[item,item],total:2}])assert.equal(valid(bad,1,20),false,JSON.stringify(bad));
 assert.equal(valid({...page,page:2},2,20),false);
 for(const [p,size] of [[0,20],[1,0],[1,101],[1.5,20],[Number.MAX_SAFE_INTEGER,100]])assert.equal(valid({...page,page:p,pageSize:size},p,size),false);
});
