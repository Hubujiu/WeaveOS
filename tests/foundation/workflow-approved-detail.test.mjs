import test from 'node:test';import assert from 'node:assert/strict';import {readFile} from 'node:fs/promises';import {stripTypeScriptTypes} from 'node:module';
const base=new URL('../../apps/web/src/applications/workflows/',import.meta.url),url=s=>'data:text/javascript;base64,'+Buffer.from(s).toString('base64');
const read=async p=>stripTypeScriptTypes(await readFile(new URL(p,base),'utf8'),{mode:'transform'});
const history=url(await read('historyContract.ts'));
const {validPersonalApprovalDetail:valid}=await import(url((await read('approvedDetailContract.ts')).replaceAll("'./historyContract'",JSON.stringify(history))));
const id=n=>'cccccccc-cccc-4ccc-8ccc-'+String(n).padStart(12,'0'),actor=id(1),eventId=id(2);
const detail=()=>({event:{id:eventId,instanceId:id(3),flowId:id(4),flowName:'原审批',flowNameSource:'captured',nodeId:id(5),targetNodeId:null,actorId:actor,action:'agree',outcome:'success',sequence:2,schemaVersion:1,recordVersion:3,occurredAt:'2026-10-13T15:00:00Z'},basis:{status:'available',fields:[{fieldId:id(6),fieldName:'原金额',fieldKind:'money',value:'123456789012345678901234567890.12',valueLabels:{}}]}});
test('personal approved detail accepts precise original own successful event and empty or unavailable basis',()=>{
 for(const action of ['agree','reject','return']){const d=detail();d.event.action=action;if(action==='return')d.event.targetNodeId=id(7);const before=structuredClone(d);assert.equal(valid(d,eventId,actor),true);assert.deepEqual(d,before)}
 for(const status of ['available','unavailable'])assert.equal(valid({...detail(),basis:{status,fields:[]}},eventId,actor),true);
});
test('personal approved detail binds original route event and current actor, never borrowed pending or no-effect events',()=>{
 for(const patch of [{id:id(9)},{actorId:id(10)},{action:'start',nodeId:null},{action:'withdraw',nodeId:null},{outcome:'no_effect'},{outcome:'pending'},{recordVersion:2**53},{flowNameSource:'current'}]){const d=detail();Object.assign(d.event,patch);assert.equal(valid(d,eventId,actor),false,JSON.stringify(patch))}
 assert.equal(valid(detail(),'BAD',actor),false);assert.equal(valid(detail(),eventId,'BAD'),false);
});
test('personal approved detail retains original closed basis validation and rejects private or fabricated values',()=>{
 for(const value of [null,[],{...detail(),currentValues:{}},{...detail(),basis:{status:'unavailable',fields:detail().basis.fields}},{...detail(),basis:{status:'available',fields:[{...detail().basis.fields[0],value:123.12}]}},{...detail(),basis:{status:'available',fields:[{...detail().basis.fields[0],valueLabels:{[id(20)]:{label:'Hidden',deleted:false}}}]}}])assert.equal(valid(value,eventId,actor),false);
});
