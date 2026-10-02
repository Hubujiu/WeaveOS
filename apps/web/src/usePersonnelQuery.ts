import {useEffect,useRef,useState} from 'react';
import {workspaceApi,WorkspaceError} from './workspace-api';
import type {MemberSearchInput,EventSearchInput,QueryPage,QueryRange} from './query-contracts';

type Result<Row> = QueryPage<Row> & {range?:QueryRange};
export function usePersonnelQuery<Row>(view:'members'|'events',parameters:MemberSearchInput|EventSearchInput){
 const [data,setData]=useState<Result<Row>>({items:[],total:0,page:1,pageSize:20,queryVersion:'',sort:null});
 const [loading,setLoading]=useState(true),[error,setError]=useState<WorkspaceError|null>(null),[epoch,setEpoch]=useState(0);
 const sequence=useRef(0),version=useRef<string|undefined>(undefined),range=useRef<{key:string;value:QueryRange}|null>(null);
 const prepared=useRef<{key:string;view:string;epoch:number}|null>(null),controllerRef=useRef<AbortController|null>(null);
 const key=JSON.stringify(parameters);
 useEffect(()=>{
  if(prepared.current?.key===key&&prepared.current.view===view&&prepared.current.epoch===epoch){prepared.current=null;return;}
  prepared.current=null;
  const input=JSON.parse(key) as MemberSearchInput|EventSearchInput;
  const token=++sequence.current,controller=new AbortController();
  controllerRef.current=controller;
  const timeKey=JSON.stringify(['from'in input?input.from:null,'to'in input?input.to:null]);
  const fixedRange=view==='events'&&version.current&&range.current?.key===timeKey?range.current.value:undefined;
  const body={...input,...fixedRange,...(version.current?{queryVersion:version.current}:{})};
  setLoading(true);setError(null);
  workspaceApi<Result<Row>>('personnel/'+view+'/search','POST',body,controller.signal).then(value=>{
   if(token!==sequence.current)return;
   version.current=value.queryVersion;
   if(value.range)range.current={key:timeKey,value:value.range};
   setData(value);
  }).catch(e=>{if(token===sequence.current&&!controller.signal.aborted)setError(e instanceof WorkspaceError?e:new WorkspaceError(0,'COMMON_UNAVAILABLE'));})
   .finally(()=>{if(token===sequence.current)setLoading(false);});
  return()=>{sequence.current++;controller.abort();};
 },[view,key,epoch]);
 function refresh(){sequence.current++;version.current=undefined;range.current=null;setError(null);setEpoch(value=>value+1);}
 function recheck(){setEpoch(value=>value+1);}
 function reject(e:WorkspaceError){setError(e);}
 // Query preparation retains the current rows/filter/visibility until the old
 // query context accepts the next filter. Its receipt prevents a duplicate
 // request when the caller commits controlled parameters in the same batch.
 async function prepareChange(input:MemberSearchInput|EventSearchInput){
  if(error||loading||!version.current)throw error??new WorkspaceError(409,'COMMON_QUERY_CONTEXT_EXPIRED');
  controllerRef.current?.abort();const controller=new AbortController();controllerRef.current=controller;
  const token=++sequence.current,timeKey=JSON.stringify(['from'in input?input.from:null,'to'in input?input.to:null]);
  const fixedRange=view==='events'&&range.current?.key===timeKey?range.current.value:undefined;
  setLoading(true);setError(null);
  try{
   const value=await workspaceApi<Result<Row>>('personnel/'+view+'/search','POST',{...input,...fixedRange,queryVersion:version.current},controller.signal);
   if(token!==sequence.current)return null;
   return ()=>{
    if(token!==sequence.current)return false;
    prepared.current={key:JSON.stringify(input),view,epoch};version.current=value.queryVersion;
    if(value.range)range.current={key:timeKey,value:value.range};
    setData(value);setLoading(false);return true;
   };
  }catch(e){if(token!==sequence.current||controller.signal.aborted)return null;const failure=e instanceof WorkspaceError?e:new WorkspaceError(0,'COMMON_UNAVAILABLE');setError(failure);throw failure;}
  finally{if(token===sequence.current)setLoading(false);}
 }
 return {data,loading,error,refresh,recheck,reject,prepareChange,blocked:!!error||loading||!data.queryVersion};
}
