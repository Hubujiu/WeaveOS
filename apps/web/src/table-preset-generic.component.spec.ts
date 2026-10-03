import {test,expect} from '@playwright/test';
import {validateResourceFilter,type ResourceFilterField} from './QueryFilterState';
import {blockFilter,presetBlocks,validateResourcePreset} from './TablePresetState';

// V017 PRD §2 / V015 ADR §8.6, §11.2: dynamic field IDs keep the recursive
// server AST, precise decimal text, NULL, false and option-set identity.
const amount='11111111-1111-4111-8111-111111111111';
const title='22222222-2222-4222-8222-222222222222';
const options='33333333-3333-4333-8333-333333333333';
const a='44444444-4444-4444-8444-444444444444';
const b='55555555-5555-4555-8555-555555555555';
const fields:ResourceFilterField[]=[
 {id:amount,kind:'money',operators:['eq','neq','gt','gte','lt','lte']},
 {id:title,kind:'text',operators:['eq','neq']},
 {id:options,kind:'multi_select',operators:['eq','neq'],optionIds:[a,b]},
];

test('dynamic AST preserves nested AND/OR and exact decimal strings without DNF',()=>{
 const input={operator:'and',children:[
  {operator:'or',children:[
   {operator:'and',children:[{fieldId:amount,operator:'gt',value:'9007199254740993.123456789'}]},
   {fieldId:title,operator:'eq',value:''},
  ]},
  {fieldId:options,operator:'eq',value:[b,a]},
 ]};
 const result=validateResourceFilter(fields,input);
 expect(result.issues).toEqual([]);
 expect(result.filter).toEqual(input);
});

test('runtime operators and typed values reject unknown or unauthorized fields',()=>{
 for(const input of [
  {operator:'and',children:[{fieldId:amount,operator:'gt',value:1.25}]},
  {operator:'and',children:[{fieldId:title,operator:'gt',value:'x'}]},
  {operator:'and',children:[{fieldId:'66666666-6666-4666-8666-666666666666',operator:'eq',value:'secret'}]},
  {operator:'and',children:[{fieldId:options,operator:'eq',value:[a,'77777777-7777-4777-8777-777777777777']}]},
 ]) expect(validateResourceFilter(fields,input).issues.length).toBeGreaterThan(0);
});

test('NULL, false and precise dates remain distinct, with server-safe depth limits',()=>{
 const date='88888888-8888-4888-8888-888888888888';
 const bool='99999999-9999-4999-8999-999999999999';
 const available:ResourceFilterField[]=[...fields,{id:date,kind:'date',operators:['eq','neq','gt','gte','lt','lte']},{id:bool,kind:'boolean',operators:['eq','neq']}];
 const input={operator:'and',children:[{fieldId:title,operator:'eq',value:null},{fieldId:bool,operator:'neq',value:false},{fieldId:date,operator:'lt',value:'2026-10-03'}]};
 expect(validateResourceFilter(available,input)).toEqual({filter:input,issues:[]});
 expect(validateResourceFilter(available,{operator:'and',children:[{fieldId:date,operator:'gt',value:'2026-02-30'}]}).issues.length).toBeGreaterThan(0);
 const tooDeep={operator:'and',children:[{operator:'or',children:[{operator:'and',children:[{operator:'or',children:[{fieldId:title,operator:'eq',value:'x'}]}]}]}]};
 expect(validateResourceFilter(available,tooDeep).issues.length).toBeGreaterThan(0);
});

test('one editor serializes record fieldId without altering AND/OR grouping',()=>{
 const saved={operator:'or',children:[
  {operator:'and',children:[{fieldId:amount,operator:'gt',value:'12345678901234567890.123'}]},
  {operator:'and',children:[{fieldId:title,operator:'eq',value:''}]},
 ]};
 const blocks=presetBlocks(saved,'fieldId');
 expect(blocks.map(b=>b.rows.map(r=>r.field))).toEqual([[amount],[title]]);
 expect(blockFilter(blocks,'fieldId')).toEqual(saved);
});

test('record preset validates authorized business visibility without discarding a hidden filter',()=>{
 const filter={operator:'and',children:[{fieldId:amount,operator:'gt',value:'9007199254740993.01'}]};
 const columns=[{id:amount,name:'金额'},{id:title,name:'标题'}];
 expect(validateResourcePreset('  私人方案  ',filter,[amount],fields,columns)).toEqual({issues:[],name:'私人方案',filter});
 expect(validateResourcePreset('私人方案',filter,[amount,title],fields,columns).issues).toContain('至少保留一个业务字段');
 expect(validateResourcePreset('私人方案',filter,['removed-field'],fields,columns).issues.length).toBeGreaterThan(0);
});

test('one manager loads scoped resource presets and redacts invalid definitions',async({page})=>{
 await page.goto('/src/table-preset-generic-fixture.html');
 await page.getByRole('button',{name:/^自定义筛选/}).click();
 const panel=page.getByRole('dialog',{name:'管理自定义筛选'});
 await expect(panel.getByText('已失效')).toBeVisible();
 await expect(panel.getByText('字段权限已变化')).toBeVisible();
 await expect(panel.getByRole('button',{name:'应用已失效'})).toBeDisabled();
 await expect(panel).not.toContainText('11111111-1111-4111-8111-111111111111');
});

test('resource editor saves fieldId and decimal text through injected repository after confirmation',async({page})=>{
 await page.goto('/src/table-preset-generic-fixture.html');
 await page.getByRole('button',{name:/^自定义筛选/}).click();
 await page.getByRole('button',{name:'新增筛选'}).click();
 const dialog=page.getByRole('dialog',{name:'新增自定义筛选'});
 await dialog.getByLabel('自定义筛选名称').fill('金额方案');
 await dialog.getByRole('button',{name:'或条件'}).click();
 await dialog.getByLabel('条件 1.1 字段').selectOption('11111111-1111-4111-8111-111111111111');
 await dialog.getByLabel('条件 1.1 比较').selectOption('gt');
 await dialog.getByLabel('条件 1.1 值',{exact:true}).fill('9007199254740993.01');
 await dialog.getByRole('button',{name:'确定'}).click();
 await expect.poll(()=>page.evaluate(()=>(window as typeof window & {__resourcePresetFixture:{writes:unknown[]}}).__resourcePresetFixture.writes.length)).toBe(1);
 const state=await page.evaluate(()=>(window as typeof window & {__resourcePresetFixture:{writes:{input:{filter:unknown}}[]}}).__resourcePresetFixture);
 expect(state.writes[0].input.filter).toEqual({operator:'and',children:[{fieldId:'11111111-1111-4111-8111-111111111111',operator:'gt',value:'9007199254740993.01'}]});
 expect(JSON.parse(await page.getByLabel('fixture-state').innerText()).active).toBeNull();
});

test('shared query lifecycle validates changed criteria with old token and clears it only on refresh or actor switch',async({page})=>{
 await page.goto('/src/table-preset-generic-fixture.html');
 const requests=()=>page.evaluate(()=>(window as typeof window & {__resourceQueryFixture:{scope:string;body:{page:number;queryVersion?:string;filter:string|null}}[]}).__resourceQueryFixture);
 await expect.poll(async()=> (await requests()).length).toBe(1);
 expect((await requests())[0].body.queryVersion).toBeUndefined();
 await page.getByRole('button',{name:'跳至第4页'}).click();
 await expect.poll(async()=> (await requests()).length).toBe(2);
 expect((await requests())[1].body).toMatchObject({page:4,queryVersion:'token-actor-A:app-1:view-1'});
 await page.getByRole('button',{name:'更改条件'}).click();
 await expect.poll(async()=> (await requests()).length).toBe(3);
 expect((await requests())[2].body).toMatchObject({page:1,filter:'金额大于10',queryVersion:'token-actor-A:app-1:view-1'});
 await page.getByRole('button',{name:'显式刷新'}).click();
 await expect.poll(async()=> (await requests()).length).toBe(4);
 expect((await requests())[3].body.queryVersion).toBeUndefined();
 await page.getByRole('button',{name:'切换主体'}).click();
 await expect.poll(async()=> (await requests()).length).toBe(5);
 expect((await requests())[4].scope).toBe('actor-B:app-1:view-1');
 expect((await requests())[4].body.queryVersion).toBeUndefined();
});
