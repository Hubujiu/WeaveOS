import {useState} from 'react';
import {createRoot} from 'react-dom/client';
import {useApplicationOperation} from './useApplicationOperation';
import type {ApplicationPacket} from './recovery';

const actorId='11111111-1111-4111-8111-111111111111';
const appId='22222222-2222-4222-8222-222222222222';
const viewId='33333333-3333-4333-8333-333333333333';
const recordId='77777777-7777-4777-8777-777777777777';
const draftId='88888888-8888-4888-8888-888888888888';
const mode=new URLSearchParams(location.search).get('mode')??'create';
const resource=mode==='delete'?{kind:'draft' as const,appId,viewId,id:draftId}:mode==='edit'?{kind:'record' as const,appId,viewId,id:recordId}:{kind:'record' as const,appId,viewId,creationNonce:'99999999-9999-4999-8999-999999999999'};
const path='applications/'+appId+'/forms/'+viewId+'/'+(mode==='delete'?'drafts/'+draftId:mode==='edit'?'records/'+recordId:'records');
const events={confirmed:[] as unknown[],authLost:0,identityMismatch:0};
Object.assign(window,{__rootScopedOperation:events});
function valid(result:unknown,packet:ApplicationPacket){
 if(!result||typeof result!=='object'||Array.isArray(result))return false;
 const r=result as Record<string,unknown>;
 const fields=mode==='delete'?['operationId','id','draftVersion']:['operationId','id','recordVersion','schemaVersion','createdAt','updatedAt'];
 if(Object.keys(r).length!==fields.length||!fields.every(key=>Object.hasOwn(r,key)))return false;
 if(r.operationId!==packet.operationId||r.id!==(mode==='delete'?draftId:recordId))return false;
 if(mode==='delete')return Number.isSafeInteger(r.draftVersion)&&(r.draftVersion as number)>=1;
 return Number.isSafeInteger(r.recordVersion)&&(r.recordVersion as number)>=1&&Number.isSafeInteger(r.schemaVersion)&&(r.schemaVersion as number)>=1&&
 typeof r.createdAt==='string'&&Number.isFinite(Date.parse(r.createdAt))&&typeof r.updatedAt==='string'&&Number.isFinite(Date.parse(r.updatedAt));
}
function Fixture(){
 const [amount,setAmount]=useState('1.20');
 const operation=useApplicationOperation<unknown>({actorId,scope:'root-scoped',resource,
  confirmed:result=>events.confirmed.push({empty:result===undefined,result}),
  unauthorized:()=>{events.authLost++;},identityMismatch:()=>{events.identityMismatch++;},valid});
 const blocked=['preflight','pending','unconfirmed'].includes(operation.phase);
 return <main>
  <label>金额<input value={amount} onChange={event=>setAmount(event.target.value)} disabled={blocked}/></label>
  <button disabled={blocked} onClick={()=>operation.start({path,resource,
   method:mode==='delete'?'DELETE':mode==='edit'?'PATCH':'POST',
   expectedStatus:mode==='delete'?204:mode==='edit'?200:201,
   ...(mode==='delete'?{expectedDraftVersion:2}:{body:mode==='edit'?{expectedSchemaVersion:1,expectedRecordVersion:1,changes:{amount}}:{expectedSchemaVersion:1,values:{amount}}})})}>保存</button>
  <button disabled={operation.phase!=='unconfirmed'} onClick={operation.query}>核查原操作</button>
  <button disabled={operation.phase!=='unconfirmed'} onClick={operation.retry}>重试原操作</button>
  <output aria-label="操作阶段">{operation.phase}</output>
  <output aria-label="当前操作">{operation.packet?.operationId??''}</output>
  <p>{operation.message}</p>
 </main>;
}
createRoot(document.getElementById('root')!).render(<Fixture/>);
