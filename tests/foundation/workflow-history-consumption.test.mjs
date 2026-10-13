import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {stripTypeScriptTypes} from 'node:module';
const source=await readFile(new URL('../../apps/web/src/applications/workflows/historyContract.ts',import.meta.url),'utf8');
const {validWorkflowHistory:list,validWorkflowEventDetail:detail,historyValue,historyComparison}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(source,{mode:'transform'})).toString('base64'));
const id=n=>'99999999-9999-4999-8999-'+String(n).padStart(12,'0');
const event={id:id(1),instanceId:id(2),flowId:id(3),flowName:'报销',flowNameSource:'captured',nodeId:id(4),targetNodeId:null,actorId:id(5),action:'agree',outcome:'success',sequence:1,schemaVersion:1,recordVersion:2,occurredAt:'2026-10-11T09:00:00.123456789Z'};
const end={hasMore:false,nextPageToken:null},next={hasMore:true,nextPageToken:'c'.repeat(43)};
const money={fieldId:id(6),fieldName:'历史金额',fieldKind:'money',value:'123456789012345678901234567890.12',valueLabels:{}};
const ref={fieldId:id(7),fieldName:'旧部门',fieldKind:'department',value:id(8),valueLabels:{[id(8)]:{label:'原财务部',deleted:true}}};
const full={event,basis:{status:'available',fields:[money,ref]}};
for(const [name,data,page,size]of [
 ['confirmed',{items:[event]},end,20],['next',{items:[event]},next,1],['empty',{items:[]},end,20],['legacy',{items:[{...event,flowNameSource:'legacy_last_known'}]},end,20],
 ['no effect',{items:[{...event,outcome:'no_effect',sequence:0}]},end,20],['start',{items:[{...event,action:'start',nodeId:null}]},end,20],['return',{items:[{...event,action:'return',targetNodeId:id(9)}]},end,20],
])test('workflow history accepts '+name,()=>assert.equal(list(data,page,size),true));
for(const [name,data,page,size]of [
 ['pending',{items:[{...event,outcome:'pending'}]},end,20],['private header',{items:[{...event,evidenceHash:'secret'}]},end,20],['missing source',{items:[Object.fromEntries(Object.entries(event).filter(([k])=>k!=='flowNameSource'))]},end,20],
 ['wrong source',{items:[{...event,flowNameSource:'current'}]},end,20],['duplicate events',{items:[event,event]},end,20],['bad timestamp',{items:[{...event,occurredAt:'not-a-time'}]},end,20],['unsafe sequence',{items:[{...event,sequence:9007199254740992}]},end,20],
 ['extra total',{items:[event],total:1},end,20],['oversize',{items:[event,{...event,id:id(10)}]},end,1],['invalid node',{items:[{...event,nodeId:'bad'}]},end,20],['unexpected return target',{items:[{...event,targetNodeId:id(9)}]},end,20],
 ['missing cursor',{items:[event]},{hasMore:true,nextPageToken:null},20],['extra pagination',{items:[event]},{...end,total:1},20],['zero pageSize',{items:[]},end,0],
])test('workflow history rejects '+name,()=>assert.equal(list(data,page,size),false));
for(const [name,value]of [['original values',full],['unavailable',{event,basis:{status:'unavailable',fields:[]}}],['empty original mask',{event,basis:{status:'available',fields:[]}}],['multi order',{event,basis:{status:'available',fields:[{...ref,fieldKind:'multi_select',value:[id(9),id(8)]}]}}]])test('event basis accepts '+name,()=>assert.equal(detail(value,event),true));
for(const [name,value]of [
 ['wrong event',{...full,event:{...event,id:id(90)}}],['changed event header',{...full,event:{...event,recordVersion:3}}],['private detail',{...full,hash:'secret'}],['unavailable values',{event,basis:{status:'unavailable',fields:[money]}}],
 ['private basis',{event,basis:{...full.basis,visibleFieldIds:[id(6)]}}],['duplicate field',{event,basis:{status:'available',fields:[money,money]}}],['numeric money',{event,basis:{status:'available',fields:[{...money,value:123.12}]}}],
 ['extra field metadata',{event,basis:{status:'available',fields:[{...money,definition:{secret:true}}]}}],['hidden label',{event,basis:{status:'available',fields:[{...ref,valueLabels:{...ref.valueLabels,[id(99)]:{label:'hidden',deleted:false}}}]}}],
 ['nonreference labels',{event,basis:{status:'available',fields:[{...money,valueLabels:ref.valueLabels}]}}],['unknown kind',{event,basis:{status:'available',fields:[{...money,fieldKind:'formula'}]}}],
])test('event basis rejects '+name,()=>assert.equal(detail(value,event),false));

// V075 P6 original strings/labels/order, never fabricate a current hidden value.
for(const [name,field,expected]of [
 ['precise decimal',money,'123456789012345678901234567890.12'],
 ['tombstone reference',ref,'原财务部（已删除）'],
 ['unknown reference',{...ref,valueLabels:{}},'引用信息不可用'],
 ['null',{...money,value:null},'—'],
 ['empty text',{...money,fieldKind:'text',value:''},''],
 ['false',{...money,fieldKind:'boolean',value:false},'否'],
 ['true',{...money,fieldKind:'boolean',value:true},'是'],
 ['multi order',{...ref,fieldKind:'multi_select',value:[id(9),id(8)],valueLabels:{...ref.valueLabels,[id(9)]:{label:'先选项目',deleted:false}}},'先选项目、原财务部（已删除）'],
])test('historical display preserves '+name,()=>assert.equal(historyValue(field),expected));
for(const [name,current,expected]of [
 ['same authorized field',{...money,fieldName:'新金额',value:'200.00'},{before:'123456789012345678901234567890.12',after:'200.00',comparable:true}],
 ['hidden or deleted current field',undefined,{before:'123456789012345678901234567890.12',after:'当前不可比较',comparable:false}],
 ['different stable field',{...money,fieldId:id(90),value:'200.00'},{before:'123456789012345678901234567890.12',after:'当前不可比较',comparable:false}],
 ['incompatible current kind',{...money,fieldKind:'text',value:'200.00'},{before:'123456789012345678901234567890.12',after:'当前不可比较',comparable:false}],
])test('historical comparison handles '+name,()=>assert.deepEqual(historyComparison(money,current),expected));

// V063 public CursorPagination is opaque, not a particular random-byte encoding.
test('workflow history accepts every nonempty opaque continuation without decoding or mutation',()=>{
 for(const token of ['next','x'.repeat(44),'x'.repeat(300),'next+/= %汉']){
  const pagination={hasMore:true,nextPageToken:token};assert.equal(list({items:[event]},pagination,20),true,token);assert.equal(pagination.nextPageToken,token);
 }
});
test('workflow history still rejects empty nonstring or contradictory opaque continuation',()=>{
 for(const pagination of [{hasMore:true,nextPageToken:''},{hasMore:true,nextPageToken:7},{hasMore:true,nextPageToken:{}},{hasMore:false,nextPageToken:'next'},{hasMore:false,nextPageToken:''}])assert.equal(list({items:[event]},pagination,20),false);
});
