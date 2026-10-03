import {useState} from 'react';
import {createRoot} from 'react-dom/client';
import {RecordForm} from './RecordForm';
import {installRootRecordTransport} from './root-record-transport';
import {createNewRecordIdentity,type RecordEditorIdentity} from './recordState';
import type {MutationResult,RuntimeField,RuntimeView} from './contracts';

const actor='11111111-1111-4111-8111-111111111111',app='22222222-2222-4222-8222-222222222222',viewId='33333333-3333-4333-8333-333333333333';
const amount='44444444-4444-4444-8444-444444444444',flag='55555555-5555-4555-8555-555555555555',secret='66666666-6666-4666-8666-666666666666',reference='88888888-8888-4888-8888-888888888888',candidate='99999999-9999-4999-8999-999999999999';
const field=(id:string,name:string,kind:RuntimeField['kind'],value?:RuntimeField['default'],create=true):RuntimeField=>({id,name,kind,required:false,presentation:{helpText:null,displayTimeZone:null},input:{},...(value!==undefined?{default:value}:{}),access:{read:create?'all':'none',create,edit:create?'all':'none',history:'none'},query:{operators:['eq','neq'],sortable:false,quickSearchable:false}});
const initial:RuntimeView={appId:app,tableId:app,viewId,schemaVersion:2,viewVersion:3,policyRevision:4,fields:[field(amount,'金额','money','9007199254740993.01'),field(flag,'同意','boolean',false),field(secret,'无权字段','text','secret',false),field(reference,'申请人','member')],layout:[amount,flag,secret,reference].map((fieldId,index)=>({id:'aaaaaaaa-aaaa-4aaa-8aaa-'+String(index+1).padStart(12,'0'),kind:'field' as const,fieldId})),capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}};
const events:{saves:{operationId:string;values?:Record<string,unknown>}[];recoveries:string[];confirmations:{result:MutationResult;identity:RecordEditorIdentity}[];dirty:boolean[];discarded:number}={saves:[],recoveries:[],confirmations:[],dirty:[],discarded:0};
Object.assign(window,{__recordFormFixture:events});
installRootRecordTransport(events,'immediate',actor,'77777777-7777-4777-8777-777777777777');
function Fixture(){const [view,setView]=useState(initial),[identity]=useState(()=>createNewRecordIdentity(actor,app,viewId)),[dirty,setDirty]=useState(false);
 return <main><button onClick={()=>setView(old=>({...old,schemaVersion:old.schemaVersion+1}))}>更改结构版本</button><RecordForm view={view} identity={identity} mode="create" authorityKey={String(view.policyRevision)+':'+view.schemaVersion} loadCandidates={async(_field,request,_signal)=>({items:request.q?[{id:candidate,label:'可选成员',status:'active'}]:[],nextPageToken:null})} onUnauthorized={()=>{}} onIdentityMismatch={()=>{}} registerLeaveGuard={()=>()=>{}} onRefresh={()=>{}} onConfirmed={(result,next)=>events.confirmations.push({result,identity:next})} onDirtyChange={value=>{events.dirty.push(value);setDirty(value);}} onRequestDiscard={()=>events.discarded++} onDiscard={()=>events.discarded++}/><output aria-label="form-state">{JSON.stringify({saves:events.saves.length,recoveries:events.recoveries.length,confirmations:events.confirmations.length,dirty})}</output></main>;
}
createRoot(document.getElementById('root')!).render(<Fixture/>);
