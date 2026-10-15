import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {stripTypeScriptTypes} from 'node:module';
const encode=src=>'data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(src,{mode:'transform'})).toString('base64');
let src=await readFile(new URL('../../apps/web/src/applications/workflows/lifecycleContract.ts',import.meta.url),'utf8');
if(src.includes("from './taskPreviewContract'")){const shared=await readFile(new URL('../../apps/web/src/applications/workflows/taskPreviewContract.ts',import.meta.url),'utf8');src=src.replace("'./taskPreviewContract'",JSON.stringify(encode(shared)));}
const {validLifecyclePreview}=await import(encode(src));
const id=n=>'cccccccc-cccc-4ccc-8ccc-'+String(n).padStart(12,'0');
const loc={appId:id(1),viewId:id(2),recordId:id(3),instanceId:id(4)};
const field={id:id(5),name:'撤回时金额',kind:'money',required:false,default:null,config:{precision:38,scale:2,roundingPlaces:2,roundingMode:'HALF_UP'},presentation:{helpText:null,displayTimeZone:null}};
const fixture=()=>({basisToken:'life-basis',instance:{id:loc.instanceId,sequence:3,state:'active'},record:{id:loc.recordId,appId:loc.appId,viewId:loc.viewId,tableId:id(6),createdBy:id(7),createdAt:'2026-10-11T09:00:00Z',updatedAt:'2026-10-11T10:00:00Z',recordVersion:2,schemaVersion:1,values:{[field.id]:'123456789012345678901234567890.12'},referenceDisplays:{}},fields:[structuredClone(field)],canWithdraw:true,returnTargets:[]});
// V045 lifecycle token contract is nonempty <=256, not a guessed format.
test('lifecycle accepts exact path-bound withdrawal capability and precise masked values',()=>{const v=fixture();assert.equal(validLifecyclePreview(v,loc),true);assert.equal(v.record.values[field.id],'123456789012345678901234567890.12');});
test('lifecycle permits return-only qualification without inventing withdrawal',()=>{const v=fixture();v.canWithdraw=false;v.returnTargets=[{nodeId:id(8),taskId:id(9),activationEpoch:2}];assert.equal(validLifecyclePreview(v,loc),true);assert.equal(v.canWithdraw,false);});
test('lifecycle accepts empty visible projection without inferring hidden fields',()=>{const v=fixture();v.fields=[];v.record.values={};assert.equal(validLifecyclePreview(v,loc),true);});
test('lifecycle retains current reference tombstone labels',()=>{const v=fixture();v.fields[0]={...field,kind:'member',config:{}};v.record.values[field.id]=id(8);v.record.referenceDisplays={[field.id]:{[id(8)]:{id:id(8),label:'离职成员',deleted:true}}};assert.equal(validLifecyclePreview(v,loc),true);});
const invalid={
 'extra top key':v=>v.actorId=id(99),'missing token':v=>delete v.basisToken,'empty token':v=>v.basisToken='','oversized token':v=>v.basisToken='a'.repeat(257),
 'wrong instance':v=>v.instance.id=id(99),'terminal state':v=>v.instance.state='withdrawn','extra instance':v=>v.instance.canWithdraw=true,'negative sequence':v=>v.instance.sequence=-1,'unsafe sequence':v=>v.instance.sequence=Number.MAX_SAFE_INTEGER+1,
 'wrong record':v=>v.record.id=id(99),'wrong app':v=>v.record.appId=id(99),'wrong view':v=>v.record.viewId=id(99),'extra record':v=>v.record.privateValue='hidden','hidden field':v=>v.record.values[id(99)]='hidden',
 'wrong money type':v=>v.record.values[field.id]=123,'bad date':v=>v.record.updatedAt='2026-02-30T00:00:00Z','duplicate field':v=>v.fields.push(v.fields[0]),'bad field config':v=>v.fields[0].config.roundingMode='RANDOM',
 'string capability':v=>v.canWithdraw='true','missing capability':v=>delete v.canWithdraw,'target null':v=>v.returnTargets=null,
 'extra target':v=>v.returnTargets=[{nodeId:id(8),taskId:id(9),activationEpoch:1,actorId:id(7)}],
 'missing target task':v=>v.returnTargets=[{nodeId:id(8),activationEpoch:1}],
 'zero target epoch':v=>v.returnTargets=[{nodeId:id(8),taskId:id(9),activationEpoch:0}],
 'unsafe target epoch':v=>v.returnTargets=[{nodeId:id(8),taskId:id(9),activationEpoch:Number.MAX_SAFE_INTEGER+1}],
 'duplicate target node':v=>v.returnTargets=[{nodeId:id(8),taskId:id(9),activationEpoch:1},{nodeId:id(8),taskId:id(10),activationEpoch:2}],
 'target limit':v=>v.returnTargets=Array.from({length:101},(_,n)=>({nodeId:id(100+n),taskId:id(500+n),activationEpoch:1})),
 'field limit':v=>{v.fields=Array.from({length:201},(_,n)=>({...field,id:id(100+n)}));v.record.values=Object.fromEntries(v.fields.map(f=>[f.id,null]));},
};
for(const [name,change]of Object.entries(invalid))test('lifecycle rejects '+name,()=>{const v=fixture();change(v);assert.equal(validLifecyclePreview(v,loc),false);});
test('lifecycle rejects invalid structures or scope without throwing',()=>{for(const v of [null,[],{},true,1,''])assert.equal(validLifecyclePreview(v,loc),false);assert.equal(validLifecyclePreview(fixture(),{...loc,recordId:'bad'}),false);});
