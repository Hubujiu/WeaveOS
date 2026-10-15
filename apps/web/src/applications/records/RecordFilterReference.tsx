import {useCallback,useState,useId} from 'react';
import {applicationApiEnvelope,ApplicationError} from '../api';
import {ReferenceSelector,type LoadReferenceCandidates,type ReferenceDisplay} from '../forms/FieldRenderer';
import {filterCandidatePage} from './filterCandidates';
import type {RuntimeField} from './contracts';

type Props={actorId:string;appId:string;viewId:string;field:RuntimeField;value:unknown;label:string;onChange:(value:string)=>void;onUnauthorized:()=>void;onIdentityMismatch:()=>void};
// A display from this API is a readable record value, not an active write target.
export function RecordFilterReference({actorId,appId,viewId,field,value,onChange,onUnauthorized,onIdentityMismatch}:Props){
 const controlId=useId();
 const [selected,setSelected]=useState<ReferenceDisplay|null>(null);
 const load=useCallback<LoadReferenceCandidates>(async(request,signal)=>{
  const query=new URLSearchParams({fieldId:field.id,pageSize:String(request.pageSize)});
  if(request.q)query.set('q',request.q);if(request.pageToken)query.set('pageToken',request.pageToken);
  try{
   const raw=await applicationApiEnvelope<unknown>(actorId,`applications/${appId}/forms/${viewId}/filter-candidates?${query}`,'GET',undefined,signal,200);
   const page=filterCandidatePage(raw);if(!page)throw Error('筛选候选响应格式不正确');
   return {items:page.items.map(item=>({...item,status:'record-value' as const})),nextPageToken:page.nextPageToken};
  }catch(error){
   if(!signal.aborted){setSelected(null);if(error instanceof ApplicationError&&error.status===401)onUnauthorized();else if(error instanceof ApplicationError&&error.code==='AUTH_SESSION_CHANGED')onIdentityMismatch();}
   throw error;
  }
 },[actorId,appId,viewId,field.id,onUnauthorized,onIdentityMismatch]);
 return <ReferenceSelector purpose="record-filter" field={field} value={typeof value==='string'?value:null} referenceDisplay={selected?.id===value?selected:null} idPrefix={'record-filter-'+controlId} loadReferenceCandidates={load} onChange={(next,candidate)=>{
  if(typeof next!=='string'||candidate?.status!=='record-value')return;
  setSelected({id:candidate.id,label:candidate.label,deleted:candidate.deleted});onChange(next);
 }}/>;
}
