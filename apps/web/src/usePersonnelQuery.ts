import {useEffect,useRef,useState} from 'react';
import {workspaceApi,WorkspaceError} from './workspace-api';
import type {MemberSearchInput,EventSearchInput,QueryPage,QueryRange} from './query-contracts';

type Result<Row> = QueryPage<Row> & {range?:QueryRange};
export function usePersonnelQuery<Row>(view:'members'|'events',parameters:MemberSearchInput|EventSearchInput){
 const [data,setData]=useState<Result<Row>>({items:[],total:0,page:1,pageSize:20,queryVersion:'',sort:null});
 const [loading,setLoading]=useState(true),[error,setError]=useState<WorkspaceError|null>(null),[epoch,setEpoch]=useState(0);
 const sequence=useRef(0),version=useRef<string|undefined>(undefined),range=useRef<{key:string;value:QueryRange}|null>(null);
 const key=JSON.stringify(parameters);
 useEffect(()=>{
  const input=JSON.parse(key) as MemberSearchInput|EventSearchInput;
  const token=++sequence.current,controller=new AbortController();
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
 return {data,loading,error,refresh,recheck,reject,blocked:!!error||loading||!data.queryVersion};
}
