import {useState} from 'react';
import {createRoot} from 'react-dom/client';
import {RecordForm} from './RecordForm';
import {LeaveGuards} from '../shell/leaveGuards';
import type {RecordItem,RuntimeField,RuntimeView} from './contracts';
import '../../style.css';
import '../../workspace.css';
import '../applications.css';

const actorId='11111111-1111-4111-8111-111111111111',other='aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',appId='22222222-2222-4222-8222-222222222222',viewId='33333333-3333-4333-8333-333333333333';
const title='44444444-4444-4444-8444-444444444444',amount='55555555-5555-4555-8555-555555555555',flag='66666666-6666-4666-8666-666666666666',denied='99999999-9999-4999-8999-999999999999';
const recordId='77777777-7777-4777-8777-777777777777',clientDraftId='88888888-8888-4888-8888-888888888888';
const field=(id:string,name:string,kind:RuntimeField['kind'],value?:RuntimeField['default'],allow=true):RuntimeField=>({id,name,kind,required:false,presentation:{helpText:null,displayTimeZone:null},input:{},...(value!==undefined?{default:value}:{}),access:{create:allow,read:allow?'all':'none',edit:allow?'all':'none',history:'none'},query:{operators:['eq','neq'],sortable:false,quickSearchable:false}});
const initial:RuntimeView={appId,viewId,tableId:appId,schemaVersion:2,viewVersion:3,policyRevision:4,fields:[
 field(title,'事由','text'),field(amount,'金额','money','9007199254740993.01'),field(flag,'同意','boolean',false),field(denied,'无权字段','text','secret',false)],
 layout:[{id:'aaaaaaaa-aaaa-4aaa-8aaa-000000000001',kind:'group',title:'申请信息',children:[title,amount,flag,denied].map((fieldId,i)=>({id:'aaaaaaaa-aaaa-4aaa-8aaa-'+String(i+2).padStart(12,'0'),kind:'field' as const,fieldId}))},
 {id:'aaaaaaaa-aaaa-4aaa-8aaa-000000000006',kind:'description',text:'<img src=x onerror=alert(1)>仅用于说明'},
 {id:'aaaaaaaa-aaaa-4aaa-8aaa-000000000007',kind:'system_field',fieldId:'recordVersion'}],
 capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}};
const record:RecordItem={id:recordId,appId,tableId:appId,viewId,createdBy:actorId,createdAt:'2026-10-03T09:00:00Z',updatedAt:'2026-10-03T09:01:00Z',recordVersion:4,schemaVersion:2,values:{[title]:'原事由',[amount]:'9007199254740993.01',[flag]:false},referenceDisplays:{}};
const mode=new URLSearchParams(location.search).get('mode')==='edit'?'edit':'create';
const guards=new LeaveGuards();
const events={confirmations:[] as unknown[],dirty:[] as boolean[],discarded:0,refreshes:0,unauthorized:0,identityMismatch:0,confirmationGuardStatuses:[] as string[][]};
Object.assign(window,{__rootEditor:events,__rootGuardStatus:()=>guards.snapshot(actorId).map(entry=>entry.status),__rootPrepareLeave:()=>guards.prepare(guards.snapshot(actorId))});
function Fixture(){
 const [actor,setActor]=useState(actorId),[mounted,setMounted]=useState(true),[view,setView]=useState(initial);
 const identity=mode==='edit'?{kind:'record' as const,actorId:actor,appId,viewId,recordId}:{kind:'new' as const,actorId:actor,appId,viewId,clientDraftId};
 return <div className="workspace"><main className="app-content" style={{height:'100dvh'}}>
 <div><button onClick={()=>setMounted(v=>!v)}>切换挂载</button><button onClick={()=>setActor(other)}>切换身份</button><button onClick={()=>setView(v=>({...v,policyRevision:v.policyRevision+1,fields:v.fields.map(f=>f.id===title?{...f,access:{...f.access,read:'none',create:false,edit:'none'}}:f)}))}>撤销事由权限</button></div>
 {mounted&&<RecordForm view={view} identity={identity} record={mode==='edit'?record:undefined} mode={mode} authorityKey={actor+':'+view.policyRevision} queryVersion="root-query-1"
 registerLeaveGuard={(scope,controller)=>guards.register(scope,controller)}
 onUnauthorized={()=>{events.unauthorized++;}} onIdentityMismatch={()=>{events.identityMismatch++;}}
 onRefresh={()=>{events.refreshes++;}}
 onConfirmed={(result,next)=>{events.confirmationGuardStatuses.push(guards.snapshot(actor).map(entry=>entry.status));events.confirmations.push({result,identity:next});}} onDirtyChange={dirty=>events.dirty.push(dirty)} onRequestDiscard={()=>{events.discarded++;}} onDiscard={()=>{events.discarded++;}}/>}
 </main></div>;
}
createRoot(document.getElementById('root')!).render(<Fixture/>);
