import test from 'node:test';import assert from 'node:assert/strict';import {readFile} from 'node:fs/promises';import {stripTypeScriptTypes} from 'node:module';
const base=new URL('../../apps/web/src/applications/',import.meta.url),url=s=>'data:text/javascript;base64,'+Buffer.from(s).toString('base64');
const read=async p=>stripTypeScriptTypes(await readFile(new URL(p,base),'utf8'),{mode:'transform'});
const records=url(await read('recordReadContracts.ts'));
const {parseComparisonSnapshot:parse}=await import(url((await read('workflows/comparisonSnapshot.ts')).replaceAll("'../recordReadContracts'",JSON.stringify(records))));
const id=n=>'cccccccc-cccc-4ccc-8ccc-'+String(n).padStart(12,'0');const actor=id(1),loc={appId:id(2),viewId:id(3),recordId:id(4)};
function fixture(){
 const field=(n,kind,input={})=>({id:id(n),name:'字段'+n,kind,required:false,presentation:{helpText:null,displayTimeZone:null},input,access:{read:'all',create:false,edit:'none',history:'all'},query:{operators:['eq'],sortable:false,quickSearchable:false}});
 const fields=[field(10,'money',{decimal:{precision:38,scale:2,roundingPlaces:2,roundingMode:'HALF_UP'}}),field(11,'department',{referenceKind:'department'}),field(12,'multi_select',{options:[{id:id(30),label:'选项A'},{id:id(31),label:'选项B'}]}),field(13,'text'),field(14,'text')];fields[4].access.read='none';
 const view={appId:loc.appId,viewId:loc.viewId,tableId:id(5),schemaVersion:2,viewVersion:3,policyRevision:4,fields,layout:[],capabilities:{create:false,read:'all',edit:'none',history:'all',search:true,draftCreate:false,draftEdit:false}};
 const record={id:loc.recordId,appId:loc.appId,viewId:loc.viewId,tableId:id(5),createdBy:actor,createdAt:'2026-10-13T12:00:00Z',updatedAt:'2026-10-13T13:00:00Z',recordVersion:5,schemaVersion:2,values:{[id(10)]:'123456789012345678901234567890.12',[id(11)]:id(20),[id(12)]:[id(31),id(30)],[id(13)]:null},referenceDisplays:{[id(11)]:{[id(20)]:{id:id(20),label:'原部门',deleted:true}}}};return {view,record};
}
test('comparison snapshot projects only actual current values with exact decimals and independently labelled references',()=>{
 const {view,record}=fixture(),before=structuredClone({view,record});
 assert.deepEqual(parse(view,record,loc,actor),{recordVersion:5,updatedAt:'2026-10-13T13:00:00Z',fields:[
 {fieldId:id(10),fieldName:'字段10',fieldKind:'money',value:'123456789012345678901234567890.12',valueLabels:{}},
 {fieldId:id(11),fieldName:'字段11',fieldKind:'department',value:id(20),valueLabels:{[id(20)]:{label:'原部门',deleted:true}}},
 {fieldId:id(12),fieldName:'字段12',fieldKind:'multi_select',value:[id(31),id(30)],valueLabels:{[id(31)]:{label:'选项B',deleted:false},[id(30)]:{label:'选项A',deleted:false}}},
 {fieldId:id(13),fieldName:'字段13',fieldKind:'text',value:null,valueLabels:{}}]});assert.deepEqual({view,record},before);
});
test('comparison snapshot refuses mismatched route record, table, schema and current actor read scope',()=>{
 for(const patch of [{id:id(90)},{appId:id(90)},{viewId:id(90)},{tableId:id(90)},{schemaVersion:3},{recordVersion:0}]){const {view,record}=fixture();Object.assign(record,patch);assert.equal(parse(view,record,loc,actor),null,JSON.stringify(patch))}
 const {view,record}=fixture();view.capabilities.read='own';record.createdBy=id(99);assert.equal(parse(view,record,loc,actor),null);assert.equal(parse(view,record,loc,'BAD'),null);
});
test('comparison snapshot rejects values outside current field read coverage rather than copying hidden values',()=>{
 for(const mode of ['none','own']){const {view,record}=fixture();view.fields[0].access.read=mode;record.createdBy=id(99);assert.equal(parse(view,record,loc,actor),null)}
 const {view,record}=fixture();record.values[id(14)]='private';assert.equal(parse(view,record,loc,actor),null);
});
test('comparison snapshot keeps absent fields absent and never fabricates missing reference labels',()=>{
 const {view,record}=fixture();delete record.values[id(13)];record.referenceDisplays={};const got=parse(view,record,loc,actor);assert.ok(got);assert.equal(got.fields.some(f=>f.fieldId===id(13)),false);assert.deepEqual(got.fields.find(f=>f.fieldId===id(11)).valueLabels,{});
});
