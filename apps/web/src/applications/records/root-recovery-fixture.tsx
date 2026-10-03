import {createRoot} from 'react-dom/client';
import {RecordForm} from './RecordForm';
import {installRootRecordTransport} from './root-record-transport';
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
const events={saves:[] as {operationId:string;values?:Record<string,unknown>}[],recoveries:[] as string[],confirmations:[] as unknown[],discarded:0};
Object.assign(window,{__rootRecovery:events});
const view:RuntimeView={
 appId:app,tableId:app,viewId,schemaVersion:2,viewVersion:3,policyRevision:4,
 fields:[{id:fieldId,name:'事由',kind:'text',required:false,presentation:{helpText:null,displayTimeZone:null},input:{},access:{read:'all',create:true,edit:'all',history:'none'},query:{operators:['eq','neq'],sortable:false,quickSearchable:false}}],
 layout:[{id:'aaaaaaaa-aaaa-4aaa-8aaa-000000000001',kind:'field',fieldId}],capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}
};
installRootRecordTransport(events,scenario,actor,recordId);
createRoot(document.getElementById('root')!).render(<RecordForm
 view={view} identity={createNewRecordIdentity(actor,app,viewId)} mode="create" authorityKey="root-authority-4"
 onUnauthorized={()=>{throw new Error('unexpected authentication failure');}}
 onIdentityMismatch={()=>{throw new Error('unexpected identity mismatch');}}
 registerLeaveGuard={()=>()=>{}} onRefresh={()=>{}}
 onConfirmed={(result,identity)=>events.confirmations.push({result,identity})}
 onDirtyChange={()=>{}} onDiscard={()=>{events.discarded++;}}
/>);
