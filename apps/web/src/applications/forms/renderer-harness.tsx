import {useState} from 'react';
import {createRoot} from 'react-dom/client';
import {FieldRenderer,type FieldValue,type ReferenceCandidatePage} from './FieldRenderer';
import {exposeHarnessGuard,registerHarnessGuard} from './guardHarness';

type RuntimeField={id:string;name:string;kind:'member';required:boolean;
  presentation:{helpText:string|null;displayTimeZone:string|null};
  input:{referenceKind:'member'};
  access:{read:'all';create:boolean;edit:'all';history:'all'};
  query:{operators:string[];sortable:boolean;quickSearchable:boolean}};
const field:RuntimeField={id:'field-member',name:'负责人',kind:'member',required:false,
  presentation:{helpText:null,displayTimeZone:null},input:{referenceKind:'member'},
  access:{read:'all',create:true,edit:'all',history:'all'},
  query:{operators:[],sortable:false,quickSearchable:false}};
const requests:{q:string;pageToken:string|null}[]=[];
let holdMore=false,moreStarted=false,moreAborted=false,releaseMore=()=>{};
const controls=window as Window&{__referenceRequests?:typeof requests;__scopeIsolation?:()=>boolean;
  __holdMore?:()=>void;__releaseMore?:()=>void;__moreState?:()=>{started:boolean;aborted:boolean}};
exposeHarnessGuard(window);
controls.__referenceRequests=requests;
controls.__holdMore=()=>{holdMore=true;};
controls.__releaseMore=()=>releaseMore();
controls.__moreState=()=>({started:moreStarted,aborted:moreAborted});
controls.__scopeIsolation=()=>{
  const first={getStatus:()=> 'draft' as const,prepareLeave:()=>({ok:true as const})};
  const second={getStatus:()=> 'unknown' as const,prepareLeave:()=>({ok:true as const})};
  const a=registerHarnessGuard({kind:'record',actorId:'actor',appId:'app',viewId:'view',recordId:'record-a'},first);
  const b=registerHarnessGuard({kind:'record',actorId:'actor',appId:'app',viewId:'view',recordId:'record-b'},second);
  b();
  // The test harness registry must retain the other resource after one unmounts.
  const retained=(window as Window&{__formsGuardStatus?:()=>string|null}).__formsGuardStatus?.()==='draft';
  a();return retained;
};
async function load({q,pageToken}:{q:string;pageSize:number;pageToken:string|null},signal:AbortSignal):Promise<ReferenceCandidatePage>{
  requests.push({q,pageToken});
  if(q==='林')return {items:[{id:'member-lin',label:'林海',status:'active'}],nextPageToken:null};
  if(pageToken){
    if(holdMore){moreStarted=true;await new Promise<void>(resolve=>{
      releaseMore=resolve;
      signal.addEventListener('abort',()=>{moreAborted=true;resolve();},{once:true});
    });}
    return {items:[{id:'member-b',label:'王乙',status:'active'}],nextPageToken:null};
  }
  return {items:[{id:'member-a',label:'王甲',status:'active'}],nextPageToken:'next-page'};
}
function ReferenceHarness(){
  const [value,setValue]=useState<FieldValue>('member-deleted');
  const [readOnly,setReadOnly]=useState(false);
  const [referenceScopeKey,setReferenceScopeKey]=useState('actor-a/app/view/record-a/field-member');
  const [hasDisplay,setHasDisplay]=useState(true);
  return <><button onClick={()=>setReadOnly(!readOnly)}>切换只读</button>
    <button onClick={()=>{setReferenceScopeKey('actor-b/app/view/record-b/field-member');setHasDisplay(false);}}>切换引用作用域</button>
    <button onClick={()=>{setHasDisplay(false);setReadOnly(true);}}>撤销引用显示</button>
    <output data-testid="selected">{String(value)}</output>
    <FieldRenderer field={field} value={value} onChange={setValue} readOnly={readOnly}
      referenceScopeKey={referenceScopeKey}
      referenceDisplay={hasDisplay?{id:'member-deleted',label:'已离职成员',deleted:true}:null}
      loadReferenceCandidates={load}/>
  </>;
}
const booleanField={id:'field-boolean',name:'是否生效',kind:'boolean' as const,required:true,
  presentation:{helpText:null}};
function BooleanHarness(){
  const [value,setValue]=useState<FieldValue>(null);
  const [readOnly,setReadOnly]=useState(false);
  return <><button onClick={()=>setReadOnly(!readOnly)}>切换只读</button>
    <output data-testid="selected">{value===null?'null':String(value)}</output>
    <FieldRenderer field={booleanField} value={value} onChange={setValue} readOnly={readOnly}/>
  </>;
}
createRoot(document.getElementById('root')!).render(
  new URLSearchParams(location.search).get('mode')==='boolean'?<BooleanHarness/>:<ReferenceHarness/>);
