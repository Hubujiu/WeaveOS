import {useState} from 'react';
import {createRoot} from 'react-dom/client';
import {RecordForm,type FieldPort,type RecordSaveCommand,type SaveOutcome} from './RecordForm';
import {createNewRecordIdentity,type RecordEditorIdentity} from './recordState';
import type {MutationResult,RuntimeField,RuntimeView} from './contracts';

const actor='11111111-1111-4111-8111-111111111111',app='22222222-2222-4222-8222-222222222222',viewId='33333333-3333-4333-8333-333333333333';
const amount='44444444-4444-4444-8444-444444444444',flag='55555555-5555-4555-8555-555555555555',secret='66666666-6666-4666-8666-666666666666';
const field=(id:string,name:string,kind:RuntimeField['kind'],value?:RuntimeField['default'],create=true):RuntimeField=>({id,name,kind,required:false,presentation:{helpText:null,displayTimeZone:null},input:{},...(value!==undefined?{default:value}:{}),access:{read:create?'all':'none',create,edit:create?'all':'none',history:'none'},query:{operators:['eq','neq'],sortable:false,quickSearchable:false}});
const initial:RuntimeView={appId:app,tableId:app,viewId,schemaVersion:2,viewVersion:3,policyRevision:4,fields:[field(amount,'金额','money','9007199254740993.01'),field(flag,'同意','boolean',false),field(secret,'无权字段','text','secret',false)],layout:[],capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}};
const events:{saves:RecordSaveCommand[];recoveries:string[];confirmations:{result:MutationResult;identity:RecordEditorIdentity}[];dirty:boolean[];discarded:number}={saves:[],recoveries:[],confirmations:[],dirty:[],discarded:0};
Object.assign(window,{__recordFormFixture:events});
function renderField({field,value,readOnly,onChange}:FieldPort){return <label>{field.name}<input aria-label={field.name} disabled={readOnly} type={field.kind==='boolean'?'checkbox':'text'} {...(field.kind==='boolean'?{checked:value===true,onChange:(e:React.ChangeEvent<HTMLInputElement>)=>onChange(e.target.checked)}:{value:typeof value==='string'?value:'',onChange:(e:React.ChangeEvent<HTMLInputElement>)=>onChange(e.target.value)})}/></label>;}
function Fixture(){const [view,setView]=useState(initial),[identity]=useState(()=>createNewRecordIdentity(actor,app,viewId));
 const save=async(command:RecordSaveCommand):Promise<SaveOutcome>=>{events.saves.push(command);return {kind:'unknown'};};
 const recover=async(operationId:string):Promise<SaveOutcome>=>{events.recoveries.push(operationId);return {kind:'confirmed',result:{operationId,id:'77777777-7777-4777-8777-777777777777',recordVersion:1,schemaVersion:2,createdAt:'2026-10-03T09:00:00Z',updatedAt:'2026-10-03T09:00:00Z'}};};
 return <main><button onClick={()=>setView(old=>({...old,schemaVersion:old.schemaVersion+1}))}>更改结构版本</button><RecordForm view={view} identity={identity} mode="create" renderField={renderField} onSave={save} onRecover={recover} onConfirmed={(result,next)=>events.confirmations.push({result,identity:next})} onDirtyChange={dirty=>events.dirty.push(dirty)} onDiscard={()=>events.discarded++}/><output aria-label="form-state">{JSON.stringify({saves:events.saves.length,recoveries:events.recoveries.length,confirmations:events.confirmations.length,dirty:events.dirty.at(-1)})}</output></main>;
}
createRoot(document.getElementById('root')!).render(<Fixture/>);
