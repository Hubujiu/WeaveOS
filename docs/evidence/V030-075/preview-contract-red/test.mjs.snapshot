import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {stripTypeScriptTypes} from 'node:module';
const src=await readFile(new URL('../../apps/web/src/applications/workflows/taskPreviewContract.ts',import.meta.url),'utf8');
const {validTaskPreview}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(src,{mode:'transform'})).toString('base64'));
const id=n=>'44444444-4444-4444-8444-'+String(n).padStart(12,'0');
const loc={appId:id(1),viewId:id(2),recordId:id(3),instanceId:id(4),taskId:id(5)};
const textField={id:id(8),name:'事由',kind:'text',required:false,default:null,config:{maxLength:null},presentation:{helpText:null,displayTimeZone:null}};
const original={basisToken:'a'.repeat(43),task:{id:loc.taskId,instanceId:loc.instanceId,nodeId:id(6),activationEpoch:1,sequence:0},record:{id:loc.recordId,appId:loc.appId,viewId:loc.viewId,tableId:id(7),createdBy:id(9),createdAt:'2026-10-11T09:00:00Z',updatedAt:'2026-10-11T10:00:00Z',recordVersion:3,schemaVersion:1,values:{[textField.id]:'独立需求样例'},referenceDisplays:{}},fields:[textField],editableFieldIds:[]};
const fixture=()=>structuredClone(original);
test('preview accepts a closed current path-bound authorized projection',()=>assert.equal(validTaskPreview(fixture(),loc),true));
const decimal={precision:38,scale:2,roundingPlaces:2,roundingMode:'HALF_UP'};
for(const [kind,config,value] of [
 ['text',{maxLength:null},'plain'],['multiline',{maxLength:100},'first\nsecond'],['number',decimal,'123456789012345678901234567890.12'],['money',decimal,'0.00'],['date',{},'2024-02-29'],['datetime',{precision:'second'},'2026-10-11T09:00:00+08:00'],['boolean',{},false],['single_select',{options:[{id:id(10),label:'原选项'}]},id(10)],['multi_select',{options:[{id:id(10),label:'原选项'}]},[id(10)]],['member',{},id(11)],['department',{},id(12)],
])test('preview keeps typed '+kind+' value without coercion',()=>{
 const v=fixture();v.fields[0]={...textField,kind,config};v.record.values[textField.id]=value;
 assert.equal(validTaskPreview(v,loc),true);assert.deepEqual(v.record.values[textField.id],value);
});
test('preview accepts null values and visible-only editable IDs',()=>{const v=fixture();v.record.values[textField.id]=null;v.editableFieldIds=[textField.id];assert.equal(validTaskPreview(v,loc),true);});
test('preview accepts authorized member and selected-option tombstones',()=>{
 const v=fixture();v.fields=[{...textField,kind:'single_select',config:{options:[]}}];v.record.values[textField.id]=id(10);v.record.referenceDisplays={[textField.id]:{[id(10)]:{id:id(10),label:'原选项',deleted:true}}};assert.equal(validTaskPreview(v,loc),true);
});
const invalid={
 'extra envelope field':v=>v.extra={},'short token':v=>v.basisToken='a','nonopaque token':v=>v.basisToken='/'.repeat(43),
 'wrong task':v=>v.task.id=id(90),'wrong instance':v=>v.task.instanceId=id(90),'zero node':v=>v.task.nodeId='00000000-0000-0000-0000-000000000000',
 'zero epoch':v=>v.task.activationEpoch=0,'negative sequence':v=>v.task.sequence=-1,'unsafe sequence':v=>v.task.sequence=9007199254740992,'extra task key':v=>v.task.assigneeId=id(9),
 'wrong app':v=>v.record.appId=id(90),'wrong view':v=>v.record.viewId=id(90),'wrong record':v=>v.record.id=id(90),'missing record version':v=>delete v.record.recordVersion,
 'bad calendar timestamp':v=>v.record.updatedAt='2026-02-30T00:00:00Z','extra record key':v=>v.record.secret='private',
 'duplicate field':v=>v.fields.push(v.fields[0]),'missing field value':v=>v.record.values={},'hidden field value':v=>v.record.values[id(90)]='secret',
 'invalid edit mask':v=>v.editableFieldIds=[id(90)],'duplicate edit mask':v=>v.editableFieldIds=[textField.id,textField.id],
 'invalid field kind':v=>v.fields[0].kind='script','missing field name':v=>delete v.fields[0].name,'invalid required':v=>v.fields[0].required='yes','extra field key':v=>v.fields[0].secret='private',
 'unknown config':v=>v.fields[0].config.eval='code','missing presentation':v=>delete v.fields[0].presentation,
 'numeric coercion':v=>{v.fields[0]={...textField,kind:'money',config:decimal};v.record.values[textField.id]=123.45;},
 'string boolean':v=>{v.fields[0]={...textField,kind:'boolean',config:{}};v.record.values[textField.id]='false';},
 'invalid date':v=>{v.fields[0]={...textField,kind:'date',config:{}};v.record.values[textField.id]='2026-02-30';},
 'unrelated display':v=>v.record.referenceDisplays[id(90)]={},
 'mismatched display ID':v=>{v.fields[0]={...textField,kind:'member',config:{}};v.record.values[textField.id]=id(10);v.record.referenceDisplays={[textField.id]:{[id(10)]:{id:id(11),label:'Private',deleted:false}}};},
};
for(const [name,change] of Object.entries(invalid))test('preview rejects '+name,()=>{const v=fixture();change(v);assert.equal(validTaskPreview(v,loc),false);});
test('preview rejects null, arrays and invalid path rather than throwing',()=>{
 for(const v of [null,[],{},true,1,''])assert.equal(validTaskPreview(v,loc),false);
 assert.equal(validTaskPreview(fixture(),{...loc,taskId:'invalid'}),false);
});
