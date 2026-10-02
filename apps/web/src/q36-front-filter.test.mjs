import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import ts from 'typescript';
const source=await readFile(new URL('./QueryFilterState.ts',import.meta.url),'utf8');
const js=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText;
const {validateQueryFilter}=await import('data:text/javascript;base64,'+Buffer.from(js).toString('base64'));
const leaf=(field='account',value='Alice',operator='eq')=>({field,operator,value});
test('Q36 root absence and canonical nested DTO preserve exact text and null',()=>{
  assert.deepEqual(validateQueryFilter('members',{operator:'and',children:[]}),{issues:[]});
  const tree={operator:'and',children:[leaf(),{operator:'or',children:[leaf('status',null,'neq'),leaf('identityIds','00000000-0000-4000-8000-000000000001','neq')]}]};
  assert.deepEqual(validateQueryFilter('members',tree),{issues:[],filter:tree});
});
test('Q36 validates all 20 leaves and 3 group levels globally',()=>{
  assert.equal(validateQueryFilter('members',{operator:'and',children:Array.from({length:21},()=>leaf())}).issues.length>0,true);
  assert.equal(validateQueryFilter('members',{operator:'and',children:[{operator:'or',children:[]}]}).issues.length>0,true);
  const deep={operator:'and',children:[{operator:'or',children:[{operator:'and',children:[{operator:'and',children:[leaf()]}]}]}]};
  assert.equal(validateQueryFilter('members',deep).issues.length>0,true);
});
test('Q36 rejects wrong view fields/operators, invalid IDs, booleans, timezone, date and lossy instant',()=>{
  for(const invalid of [leaf('account','Alice','gt'),leaf('occurredAt','2026-10-01T00:00:00Z'),leaf('status','pending'),leaf('identityIds',null),leaf('departmentIds','not-a-uuid'),leaf('personnelManage','true')]){
    assert.ok(validateQueryFilter('members',{operator:'and',children:[invalid]}).issues.length,JSON.stringify(invalid));
  }
  for(const value of [{date:'2026-02-30',timeZone:'UTC'},{date:'2026-10-01',timeZone:'Not/AZone'},'2026-10-01T10:00:00.1234567Z','2026-02-30T10:00:00Z','2026-10-01T10:00:00']){
    assert.ok(validateQueryFilter('events',{operator:'and',children:[leaf('occurredAt',value,'gte')]}).issues.length,JSON.stringify(value));
  }
});
test('Q36 16KiB budget counts canonical UTF-8 bytes; valid fixed-zone date remains untouched',()=>{
  assert.ok(validateQueryFilter('members',{operator:'and',children:[leaf('account','界'.repeat(5500))]}).issues.length);
  const tree={operator:'or',children:[leaf('occurredAt',{date:'2026-10-01',timeZone:'Asia/Shanghai'})]};
  assert.deepEqual(validateQueryFilter('events',tree),{issues:[],filter:tree});
});
