import {test,expect} from '@playwright/test';
import {parseRuntimeView,parseRecordItem,parseRecordPage} from './recordReadContracts';
const actor='11111111-1111-4111-8111-111111111111',app='22222222-2222-4222-8222-222222222222',viewId='33333333-3333-4333-8333-333333333333';
const fieldId='44444444-4444-4444-8444-444444444444',sourceId='55555555-5555-4555-8555-555555555555',recordId='77777777-7777-4777-8777-777777777777';
const field=(kind:string,input:object={})=>({id:fieldId,name:'字段',kind,required:false,presentation:{helpText:null,displayTimeZone:null},input,access:{read:'all',create:true,edit:'all',history:'none'},query:{operators:['eq','neq'],sortable:false,quickSearchable:false}});
const rawView=(f=field('text'))=>({appId:app,tableId:app,viewId,schemaVersion:1,viewVersion:1,policyRevision:1,fields:[f],layout:[{id:'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',kind:'field',fieldId}],capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}});
const row=(value:unknown)=>({id:recordId,appId:app,tableId:app,viewId,createdBy:actor,createdAt:'2026-10-03T09:00:00Z',updatedAt:'2026-10-03T09:00:00Z',recordVersion:1,schemaVersion:1,values:{[fieldId]:value},referenceDisplays:{}});
const parseView=(raw:unknown)=>parseRuntimeView(raw,{appId:app,viewId});
const page=(items:unknown[],sort:unknown=null)=>({items,total:items.length,page:1,pageSize:20,sort,queryVersion:'opaque-query',schemaVersion:1,viewVersion:1});

for(const kind of ['member','department']){
 test('Root read contract keeps scalar '+kind+' references including tombstones',()=>{
  const view=parseView(rawView(field(kind,{referenceKind:kind})));
  const item={...row(sourceId),referenceDisplays:{[fieldId]:{[sourceId]:{id:sourceId,label:'历史成员',deleted:true}}}};
  expect(parseRecordItem(item,view,actor).values[fieldId]).toBe(sourceId);
  expect(()=>parseRecordItem(row([sourceId]),view,actor)).toThrow();
 });
}

test('Root runtime accepts rounding at the hundreds place without float conversion',()=>{
 const raw=rawView(field('money',{decimal:{precision:20,scale:2,roundingPlaces:-2,roundingMode:'TOWARD_ZERO'}}));
 const view=parseView(raw);
 expect(view.fields[0].input.decimal?.roundingPlaces).toBe(-2);
 expect(parseRecordItem(row('9007199254740900.00'),view,actor).values[fieldId]).toBe('9007199254740900.00');
});
for(const decimal of [
 {precision:0,scale:0,roundingPlaces:0},
 {precision:39,scale:2,roundingPlaces:2},
 {precision:20,scale:19,roundingPlaces:2},
 {precision:2,scale:3,roundingPlaces:2},
 {precision:20,scale:2,roundingPlaces:-19},
 {precision:20,scale:2,roundingPlaces:3},
]){
 test('Root rejects invalid decimal configuration '+JSON.stringify(decimal),()=>{
  expect(()=>parseView(rawView(field('money',{decimal:{...decimal,roundingMode:'HALF_UP'}})))).toThrow();
 });
}

test('Root record read-none and own-row policies cannot be widened by field access',()=>{
 const view=parseView(rawView());
 expect(()=>parseRecordItem(row('value'),{...view,capabilities:{...view.capabilities,read:'none'}},actor)).toThrow();
 expect(()=>parseRecordItem({...row('value'),createdBy:sourceId},{...view,capabilities:{...view.capabilities,read:'own'}},actor)).toThrow();
});
test('Root own-field value is rejected when the record belongs to somebody else',()=>{
 const raw=rawView();raw.fields[0].access.read='own';const view=parseView(raw);
 expect(parseRecordItem(row('本人值'),view,actor).values[fieldId]).toBe('本人值');
 expect(()=>parseRecordItem({...row('不属于本人的值'),createdBy:sourceId},view,actor)).toThrow();
});
test('Root persisted records require positive record and schema versions',()=>{
 const view=parseView(rawView());
 expect(()=>parseRecordItem({...row('x'),recordVersion:0},view,actor)).toThrow();
 const unready={...view,schemaVersion:0};
 expect(()=>parseRecordItem({...row('x'),schemaVersion:0},unready,actor)).toThrow();
});

test('Root supports system timestamp sort and ignores JSON object key order',()=>{
 const view=parseView(rawView());
 const sort={fieldId:'createdAt',direction:'asc'} as const;
 expect(parseRecordPage(page([row('x')],{direction:'asc',fieldId:'createdAt'}),view,actor,{page:1,pageSize:20,sort}).sort).toEqual(sort);
 const numeric=parseView(rawView(field('money')));numeric.fields[0].query.sortable=true;
 const fieldSort={fieldId,direction:'desc'} as const;
 expect(parseRecordPage(page([row('1.20')],{direction:'desc',fieldId}),numeric,actor,{page:1,pageSize:20,sort:fieldSort}).items).toHaveLength(1);
});

test('Root rejects duplicate row IDs and oversized pages',()=>{
 const view=parseView(rawView());
 expect(()=>parseRecordPage(page([row('x'),row('y')]),view,actor,{page:1,pageSize:20,sort:null})).toThrow();
 const oversized={...page([row('x')]),pageSize:101};
 expect(()=>parseRecordPage(oversized,view,actor,{page:1,pageSize:101,sort:null})).toThrow();
 const excess={...page([row('x'),{...row('y'),id:sourceId}]),pageSize:1};
 expect(()=>parseRecordPage(excess,view,actor,{page:1,pageSize:1,sort:null})).toThrow();
});

test('Root preserves description layout and canonical UUIDs without inventing version restrictions',()=>{
 const raw=rawView();
 const anyVersion='00000000-0000-0000-0000-000000000001';
 raw.fields[0].id=anyVersion;raw.layout[0].fieldId=anyVersion;
 const withDescription={...raw,layout:[{id:'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaab',kind:'description',text:'填写说明'},...raw.layout]};
 expect(parseView(withDescription).layout[0]).toMatchObject({kind:'description',text:'填写说明'});
});
test('Root calendar dates reject impossible days and retain leap days',()=>{
 const view=parseView(rawView(field('date')));
 expect(parseRecordItem(row('2024-02-29'),view,actor).values[fieldId]).toBe('2024-02-29');
 expect(()=>parseRecordItem(row('2026-02-31'),view,actor)).toThrow();
});
test('Root query operators must be literal strings rather than coercible arrays',()=>{
 const raw=rawView();const malformed={...raw,fields:[{...raw.fields[0],query:{...raw.fields[0].query,operators:[['eq']]}}]};
 expect(()=>parseView(malformed)).toThrow();
});
test('Root layout span follows the declared integer range',()=>{
 const raw=rawView();
 expect(()=>parseView({...raw,layout:[{...raw.layout[0],span:13}]})).toThrow();
 expect(()=>parseView({...raw,layout:[{...raw.layout[0],span:1.5}]})).toThrow();
 expect(parseView({...raw,layout:[{...raw.layout[0],span:6}]}).layout[0]).toMatchObject({span:6});
});
