import {test,expect} from '@playwright/test';
import {validateResourceFilter,type ResourceFilterField} from './QueryFilterState';

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
