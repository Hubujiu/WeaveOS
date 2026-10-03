import { createRoot } from 'react-dom/client';
import { useState } from 'react';
import { ApplicationStructurePanel } from './ApplicationStructurePanel';
import { FormDesigner } from './FormDesigner';
import {exposeHarnessGuard,registerHarnessGuard} from './guardHarness';

const query = new URLSearchParams(location.search);
const initialAppId = query.get('appId') ?? '';
const initialViewId = query.get('viewId') ?? '';
const initialActorId = query.get('actorId') ?? '00000000-0000-4000-8000-000000000181';
function Harness(){
  exposeHarnessGuard(window);
  const [appId,setAppId]=useState(initialAppId);
  const [viewId,setViewId]=useState(initialViewId);
  const [actorId,setActorId]=useState(initialActorId);
  (window as Window & {__formsHarnessSwitchApp?:(id:string)=>void}).__formsHarnessSwitchApp=setAppId;
  (window as Window & {__formsHarnessSwitchView?:(id:string)=>void}).__formsHarnessSwitchView=setViewId;
  (window as Window & {__formsHarnessSwitchActor?:(id:string)=>void}).__formsHarnessSwitchActor=setActorId;
  const authWindow=window as Window & {__formsHarnessAuthEvents?:string[]};
  authWindow.__formsHarnessAuthEvents??=[];
  const onUnauthorized=()=>{authWindow.__formsHarnessAuthEvents?.push('401');};
  const onIdentityMismatch=()=>{authWindow.__formsHarnessAuthEvents?.push('AUTH_SESSION_CHANGED');};
  return <>{query.get('switchViewId')&&<button type="button" onClick={()=>setViewId(query.get('switchViewId')!)}>切换视图</button>}
    {query.get('switchAppId')&&<button type="button" onClick={()=>setAppId(query.get('switchAppId')!)}>切换应用</button>}
    {query.get('mode') === 'structure'
      ? <ApplicationStructurePanel {...{appId,actorId,onUnauthorized,onIdentityMismatch}}
          registerLeaveGuard={registerHarnessGuard}/>
      : <FormDesigner {...{appId,viewId,actorId,onUnauthorized,onIdentityMismatch}}
          registerLeaveGuard={registerHarnessGuard}/>}</>;
}
createRoot(document.getElementById('root')!).render(<Harness/>);
