import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
const url=source=>'data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(source,{mode:'transform'})).toString('base64');
const read=p=>readFileSync(new URL('../../apps/web/src/'+p,import.meta.url),'utf8');
const query=url(read('QueryFilterState.ts'));
const {validateResourcePreset:validate}=await import(url(read('TablePresetState.ts').replace("from './QueryFilterState'",'from '+JSON.stringify(query))));
const {presetDescriptors:describe}=await import(url(read('applications/records/presetDescriptors.ts')));
const amount='aaaaaaaa-aaaa-4aaa-8aaa-000000000004',own='aaaaaaaa-aaaa-4aaa-8aaa-000000000005',secret='aaaaaaaa-aaaa-4aaa-8aaa-000000000006';
const field=(id,name,read,operators)=>({id,name,kind:'money',access:{read},query:{operators},input:{}});
const view={fields:[field(amount,'金额','all',['eq','gt']),field(own,'本人值','own',[]),field(secret,'秘密','none',['eq'])]};
const columns=[{id:amount,name:'金额'},{id:own,name:'本人值'},...['id','createdBy','createdAt','updatedAt','recordVersion'].map(id=>({id,name:id,system:true}))];
test('V071 descriptors include exactly readable business and five allowed display-only system columns',()=>{
 const result=describe(view);assert.deepEqual(result.columns.map(c=>c.id),[amount,own,'id','createdBy','createdAt','updatedAt','recordVersion']);
 assert.deepEqual(result.columns.filter(c=>c.system).map(c=>c.id),['id','createdBy','createdAt','updatedAt','recordVersion']);
});
test('V071 only authorized business operators and the two system dates become filter descriptors',()=>{
 const result=describe(view);assert.deepEqual(result.fields.map(f=>f.id),[amount,'createdAt','updatedAt']);assert.deepEqual(result.fields[0].operators,['eq','gt']);
 for(const field of result.fields.slice(1)){assert.equal(field.kind,'datetime');assert.deepEqual(field.operators,['eq','neq','gt','gte','lt','lte']);}
 assert.ok(!JSON.stringify(result).includes(secret));
});
test('V071 descriptors do not change current runtime and preserve enum identities',()=>{
 const input={fields:[{id:amount,name:'类别',kind:'single_select',access:{read:'all'},query:{operators:['eq','neq']},input:{options:[{id:'approved-option',label:'采购'}]}}]};const before=structuredClone(input);const result=describe(input);assert.deepEqual(input,before);assert.deepEqual(result.fields[0]?.choices,[{value:'approved-option',label:'采购'}]);assert.deepEqual(result.fields[0]?.optionIds,['approved-option']);
});
test('V071 showing system columns cannot substitute for a visible business field',()=>assert.ok(validate('方案',null,[amount,own],[],columns).issues.includes('至少保留一个业务字段')));
test('V071 all five system columns can be hidden while one business field remains',()=>assert.deepEqual(validate('方案',null,[own,'id','createdBy','createdAt','updatedAt','recordVersion'],[],columns).issues,[]));
test('V071 missing business columns cannot be made valid with system columns alone',()=>assert.ok(validate('方案',null,[],[],columns.filter(c=>c.system)).issues.includes('至少保留一个业务字段')));
for(const id of ['selection','updatedBy',secret])test('V071 rejects unavailable column '+id,()=>assert.ok(validate('方案',null,[id],[],columns).issues.length));
test('V071 shared personnel column semantics remain business by default',()=>assert.ok(validate('方案',null,['account'],[],[{id:'account',name:'成员'}]).issues.includes('至少保留一个业务字段')));
