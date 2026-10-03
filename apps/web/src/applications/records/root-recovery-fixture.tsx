import {createRoot} from 'react-dom/client';
import {RecordForm,type RecordSaveCommand,type SaveOutcome} from './RecordForm';
import {createNewRecordIdentity} from './recordState';
import type {RuntimeView} from './contracts';

// Root-authored boundary fixture. Only transport outcomes are simulated;
// RecordForm and its identity/receipt checks are the real components.
const actor='11111111-1111-4111-8111-111111111111';
const app='22222222-2222-4222-8222-222222222222';
const viewId='33333333-3333-4333-8333-333333333333';
const fieldId='44444444-4444-4444-8444-444444444444';
const recordId='77777777-7777-4777-8777-777777777777';
const scenario=new URLSearchParams(location.search).get('scenario')??'failed';
const events={saves:[] as RecordSaveCommand[],recoveries:[] as string[],confirmations:[] as unknown[],discarded:0};
Object.assign(window,{__rootRecovery:events});
const view:RuntimeView={
 appId:app,tableId:app,viewId,schemaVersion:2,viewVersion:3,policyRevision:4,
 fields:[{id:fieldId,name:'事由',kind:'text',required:false,presentation:{helpText:null,displayTimeZone:null},input:{},access:{read:'all',create:true,edit:'all',history:'none'},query:{operators:['eq','neq'],sortable:false,quickSearchable:false}}],
 layout:[],capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}
};
const confirmed=(operationId:string):SaveOutcome=>({kind:'confirmed',result:{operationId,id:recordId,recordVersion:1,schemaVersion:2,createdAt:'2026-10-03T09:00:00Z',updatedAt:'2026-10-03T09:00:00Z'}});
createRoot(document.getElementById('root')!).render(<RecordForm
 view={view} identity={createNewRecordIdentity(actor,app,viewId)} mode="create" authorityKey="root-authority-4"
 onSave={async command=>{events.saves.push(command);return scenario==='initial-failure'&&events.saves.length===1?{kind:'failed',message:'字段校验失败'}:scenario==='initial-failure'?confirmed(command.operationId):{kind:'unknown'};}}
 onRecover={async operationId=>{events.recoveries.push(operationId);if(events.recoveries.length===1){if(scenario==='throw')throw new Error('network unavailable');if(scenario==='malformed')return {kind:'confirmed',result:{operationId}};return {kind:'failed',message:'恢复接口暂不可用'};}return confirmed(operationId);}}
 onConfirmed={(result,identity)=>events.confirmations.push({result,identity})}
 onDirtyChange={()=>{}} onDiscard={()=>{events.discarded++;}}
/>);
