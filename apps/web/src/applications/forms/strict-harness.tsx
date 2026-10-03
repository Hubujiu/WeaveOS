import {StrictMode,useState} from 'react';
import {createRoot} from 'react-dom/client';
import {ApplicationStructurePanel} from './ApplicationStructurePanel';
import {FormDesigner} from './FormDesigner';

const query=new URLSearchParams(location.search);
function StrictHarness(){
  const [mounted,setMounted]=useState(true);
  const [appId,setAppId]=useState(query.get('appId')??'');
  const [viewId,setViewId]=useState(query.get('viewId')??'');
  const actorId=query.get('actorId')??'';
  const controls=window as Window&{
    __formsStrictMount?:(value:boolean)=>void;
    __formsStrictApp?:(value:string)=>void;
    __formsStrictView?:(value:string)=>void;
  };
  controls.__formsStrictMount=setMounted;
  controls.__formsStrictApp=setAppId;
  controls.__formsStrictView=setViewId;
  if(!mounted)return null;
  return query.get('mode')==='structure'?<ApplicationStructurePanel appId={appId} actorId={actorId}/>:
    <FormDesigner appId={appId} viewId={viewId} actorId={actorId}/>;
}
createRoot(document.getElementById('root')!).render(<StrictMode><StrictHarness/></StrictMode>);
