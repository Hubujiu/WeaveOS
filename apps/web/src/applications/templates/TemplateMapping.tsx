import {useCallback,useState} from 'react';
import {applicationApiEnvelope,ApplicationError} from '../api';
import {ReferenceSelector,type ReferenceCandidate} from '../forms/FieldRenderer';
import {Table,type TableColumn} from '../../vendor/arca/components/motion/table';
import {templateCandidatePage} from './candidates';
import type {TemplateReferences} from './references';
import '../../vendor/arca/arca.css';
import '../../personnel-table.css';
export type TemplateSelection=Record<string,ReferenceCandidate>;
export type TemplateReference={id:string;kind:'user'|'department';sourceId:string};
export function referenceRows(references:TemplateReferences):TemplateReference[]{
 return [...references.userIds.map(sourceId=>({id:'user:'+sourceId,kind:'user' as const,sourceId})),...references.departmentIds.map(sourceId=>({id:'department:'+sourceId,kind:'department' as const,sourceId}))];
}
export function TemplateMapping({actorId,references,selection,disabled,onSelect,onUnauthorized,onIdentityMismatch}:{actorId:string;references:TemplateReferences;selection:TemplateSelection;disabled:boolean;onSelect:(reference:TemplateReference,candidate:ReferenceCandidate)=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void}){
 const [active,setActive]=useState<TemplateReference|null>(null);
 const kind=active?.kind;
 const load=useCallback(async(request:{q:string;pageSize:number;pageToken:string|null},signal:AbortSignal)=>{
  const query=new URLSearchParams({q:request.q,pageSize:String(request.pageSize)});if(request.pageToken!==null)query.set('pageToken',request.pageToken);
  try{
   const raw=await applicationApiEnvelope<unknown>(actorId,`application-templates/${kind==='department'?'department':'member'}-candidates?${query}`,'GET',undefined,signal,200);
   const page=templateCandidatePage(raw,kind??'user');if(!page)throw Error('候选响应无效');return page;
  }catch(cause){if(!signal.aborted){if(cause instanceof ApplicationError&&cause.status===401)onUnauthorized();else if(cause instanceof ApplicationError&&cause.code==='AUTH_SESSION_CHANGED')onIdentityMismatch();}throw cause;}
 },[actorId,kind,onUnauthorized,onIdentityMismatch]);
 const rows=referenceRows(references),columns:TableColumn<TemplateReference>[]=[
  {key:'source',header:'模板依赖',sortable:false,dataType:'text',cell:row=><span className="template-reference-source" title={row.sourceId}>{row.kind==='user'?'用户':'部门'}：{row.sourceId}</span>},
  {key:'target',header:'当前对象',sortable:false,dataType:'text',cell:row=><div className="template-reference-target"><span title={selection[row.id]?.label}>{selection[row.id]?.label??'未选择'}</span>{!disabled&&<button type="button" className="admin-button" onClick={()=>setActive(row)}>选择{row.kind==='user'?'成员':'部门'}</button>}</div>},
  {key:'state',header:'状态',sortable:false,dataType:'text',cell:row=>selection[row.id]?'已选择':'待选择'},
 ];
 return <section aria-label="模板引用映射" className="template-mapping">
  <div className="arca-source personnel-source-scope"><Table className="personnel-source-table" ariaLabel="模板依赖映射" data={rows} columns={columns} getRowId={row=>row.id} rowHeight={56} height={Math.min(224,44+rows.length*56)} minWidth={560} manualSorting emptyState="无外部引用"/></div>
  {active&&!disabled&&<section className="template-target-picker" aria-label={'选择当前'+(active.kind==='user'?'成员':'部门')}>
   <div className="template-target-heading"><span>为 {active.sourceId} 选择当前{active.kind==='user'?'成员':'部门'}</span><button type="button" className="admin-button" onClick={()=>setActive(null)}>取消选择</button></div>
   <ReferenceSelector key={active.id} initiallyOpen field={{id:active.sourceId,name:'当前对象',kind:active.kind==='user'?'member':'department',required:true,presentation:{helpText:null}}} value={null} idPrefix={'template-'+active.kind} loadReferenceCandidates={load} onChange={(_value,candidate)=>{if(candidate){onSelect(active,candidate);setActive(null);}}}/>
  </section>}
 </section>;
}
