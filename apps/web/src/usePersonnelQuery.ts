import {useEffect,useRef,useState} from 'react';
import {workspaceApi,WorkspaceError} from './workspace-api';
import type {MemberSearchInput,EventSearchInput,QueryPage,QueryRange} from './query-contracts';

type Result<Row> = QueryPage<Row> & {range?:QueryRange};
type ResourcePage = {items:unknown[];total:number;page:number;pageSize:number;queryVersion:string;range?:QueryRange};
type QueryTransport<Page,Input extends object> = (body:Input&{queryVersion?:string},signal:AbortSignal)=>Promise<Page>;

// One query lifecycle for personnel and application records. The resource key
// includes the verified actor and full resource scope. Only an explicit
// refresh drops a valid old token within that scope.
export function useResourceQuery<Page extends ResourcePage,Input extends object,Failure extends Error=Error>(
 resourceKey:string,parameters:Input,initialPage:Page,transport:QueryTransport<Page,Input>,
 normalizeFailure:(error:unknown)=>Failure=(error=>error instanceof Error?error:new Error('查询失败')) as (error:unknown)=>Failure,
 options:{freezeRange?:boolean}={},
){
 const [dataState,setDataState]=useState<{scope:string;value:Page}>({scope:resourceKey,value:initialPage});
 const [errorState,setErrorState]=useState<{scope:string;value:Failure|null}>({scope:resourceKey,value:null});
 const [loadingState,setLoadingState]=useState<{scope:string;value:boolean}>({scope:resourceKey,value:true});
 const [epoch,setEpoch]=useState(0);
 const sequence=useRef(0),version=useRef<string|undefined>(undefined),range=useRef<{key:string;value:QueryRange}|null>(null);
 const prepared=useRef<{key:string;scope:string;epoch:number}|null>(null),controllerRef=useRef<AbortController|null>(null);
 const currentScope=useRef(resourceKey),transportRef=useRef(transport),normalizeRef=useRef(normalizeFailure);
 transportRef.current=transport;normalizeRef.current=normalizeFailure;
 if(currentScope.current!==resourceKey){
  currentScope.current=resourceKey;sequence.current++;controllerRef.current?.abort();
  version.current=undefined;range.current=null;prepared.current=null;
 }
 const data=dataState.scope===resourceKey?dataState.value:initialPage;
 const error=errorState.scope===resourceKey?errorState.value:null;
 const loading=loadingState.scope===resourceKey?loadingState.value:true;
 const key=JSON.stringify(parameters);
 const rangeKey=(input:Input)=>JSON.stringify([('from'in input?input.from:null),('to'in input?input.to:null)]);
 const bodyFor=(input:Input)=>{
  const fixed=options.freezeRange&&version.current&&range.current?.key===rangeKey(input)?range.current.value:undefined;
  return {...input,...fixed,...(version.current?{queryVersion:version.current}:{})} as Input&{queryVersion?:string};
 };
 useEffect(()=>{
  if(prepared.current?.key===key&&prepared.current.scope===resourceKey&&prepared.current.epoch===epoch){prepared.current=null;return;}
  prepared.current=null;
  const input=JSON.parse(key) as Input,token=++sequence.current,controller=new AbortController();
  controllerRef.current=controller;
  setLoadingState({scope:resourceKey,value:true});setErrorState({scope:resourceKey,value:null});
  transportRef.current(bodyFor(input),controller.signal).then(value=>{
   if(token!==sequence.current||currentScope.current!==resourceKey)return;
   version.current=value.queryVersion;
   if(options.freezeRange&&value.range)range.current={key:rangeKey(input),value:value.range};
   setDataState({scope:resourceKey,value});
  }).catch(failure=>{
   if(token===sequence.current&&!controller.signal.aborted&&currentScope.current===resourceKey)setErrorState({scope:resourceKey,value:normalizeRef.current(failure)});
  }).finally(()=>{if(token===sequence.current&&currentScope.current===resourceKey)setLoadingState({scope:resourceKey,value:false});});
  return()=>{sequence.current++;controller.abort();};
 },[resourceKey,key,epoch]);
 function refresh(){sequence.current++;controllerRef.current?.abort();version.current=undefined;range.current=null;setErrorState({scope:resourceKey,value:null});setEpoch(value=>value+1);}
 function recheck(){setEpoch(value=>value+1);}
 function reject(failure:Failure){setErrorState({scope:resourceKey,value:failure});}
 // A preflight receipt preserves current rows and controlled inputs until the
 // old query context accepts the candidate. It is invalid after scope changes.
 async function prepareChange(input:Input){
  if(error||loading||!version.current)throw error??normalizeRef.current(new WorkspaceError(409,'COMMON_QUERY_CONTEXT_EXPIRED'));
  controllerRef.current?.abort();const controller=new AbortController();controllerRef.current=controller;
  const token=++sequence.current;setLoadingState({scope:resourceKey,value:true});setErrorState({scope:resourceKey,value:null});
  try{
   const value=await transportRef.current(bodyFor(input),controller.signal);
   if(token!==sequence.current||currentScope.current!==resourceKey)return null;
   return ()=>{
    if(token!==sequence.current||currentScope.current!==resourceKey)return false;
    prepared.current={key:JSON.stringify(input),scope:resourceKey,epoch};version.current=value.queryVersion;
    if(options.freezeRange&&value.range)range.current={key:rangeKey(input),value:value.range};
    setDataState({scope:resourceKey,value});setLoadingState({scope:resourceKey,value:false});return true;
   };
  }catch(failure){if(token!==sequence.current||controller.signal.aborted||currentScope.current!==resourceKey)return null;const normalized=normalizeRef.current(failure);setErrorState({scope:resourceKey,value:normalized});throw normalized;}
  finally{if(token===sequence.current&&currentScope.current===resourceKey)setLoadingState({scope:resourceKey,value:false});}
 }
 return {data,loading,error,refresh,recheck,reject,prepareChange,blocked:!!error||loading||!data.queryVersion};
}

export function usePersonnelQuery<Row>(view:'members'|'events',parameters:MemberSearchInput|EventSearchInput){
 return useResourceQuery<Result<Row>,MemberSearchInput|EventSearchInput,WorkspaceError>(
  'personnel:'+view,parameters,{items:[],total:0,page:1,pageSize:20,queryVersion:'',sort:null},
  (body,signal)=>workspaceApi<Result<Row>>('personnel/'+view+'/search','POST',body,signal),
  error=>error instanceof WorkspaceError?error:new WorkspaceError(0,'COMMON_UNAVAILABLE'),
  {freezeRange:view==='events'},
 );
}
