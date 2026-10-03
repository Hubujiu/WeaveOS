import {useState} from 'react';
import {createRoot} from 'react-dom/client';
import {FieldRenderer,type FieldValue,type ReferenceCandidatePage} from './FieldRenderer';
import type {Field} from './contracts';
import {exposeHarnessGuard,registerHarnessGuard} from './guardHarness';

const field={id:'field-member',name:'负责人',kind:'member',required:false,
  presentation:{helpText:null,displayTimeZone:null},input:{referenceKind:'member'}};
const requests:{q:string;pageToken:string|null}[]=[];
const controls=window as Window&{__referenceRequests?:typeof requests;__scopeIsolation?:()=>boolean};
exposeHarnessGuard(window);
controls.__referenceRequests=requests;
controls.__scopeIsolation=()=>{
  const first={getStatus:()=> 'draft' as const,prepareLeave:()=>({ok:true as const})};
  const second={getStatus:()=> 'unknown' as const,prepareLeave:()=>({ok:true as const})};
  const a=registerHarnessGuard({kind:'record',actorId:'actor',appId:'app',viewId:'view',recordId:'record-a'} as never,first);
  const b=registerHarnessGuard({kind:'record',actorId:'actor',appId:'app',viewId:'view',recordId:'record-b'} as never,second);
  b();
  // The test harness registry must retain the other resource after one unmounts.
  const retained=(window as Window&{__formsGuardStatus?:()=>string|null}).__formsGuardStatus?.()==='draft';
  a();return retained;
};
async function load({q,pageToken}:{q:string;pageSize:number;pageToken:string|null},_signal:AbortSignal):Promise<ReferenceCandidatePage>{
  requests.push({q,pageToken});
  if(q==='林')return {items:[{id:'member-lin',label:'林海',status:'active'}],nextPageToken:null};
  if(pageToken)return {items:[{id:'member-b',label:'王乙',status:'active'}],nextPageToken:null};
  return {items:[{id:'member-a',label:'王甲',status:'active'}],nextPageToken:'next-page'};
}
function Harness(){
  const [value,setValue]=useState<FieldValue>('member-deleted');
  const [readOnly,setReadOnly]=useState(false);
  return <><button onClick={()=>setReadOnly(!readOnly)}>切换只读</button>
    <output data-testid="selected">{String(value)}</output>
    <FieldRenderer field={field as unknown as Field} value={value} onChange={setValue} readOnly={readOnly}
      referenceDisplay={{id:'member-deleted',label:'已离职成员',deleted:true}}
      loadReferenceCandidates={load}/>
  </>;
}
createRoot(document.getElementById('root')!).render(<Harness/>);
