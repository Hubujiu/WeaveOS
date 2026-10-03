import {StrictMode,useState} from 'react';
import {createRoot} from 'react-dom/client';
import {ApplicationStructurePanel} from './ApplicationStructurePanel';
import {FormDesigner} from './FormDesigner';

const query=new URLSearchParams(location.search);
function StrictHarness(){
  const [mounted,setMounted]=useState(true);
  const [dirty,setDirty]=useState(false);
  const [appId,setAppId]=useState(query.get('appId')??'');
  const [viewId,setViewId]=useState(query.get('viewId')??'');
  const actorId=query.get('actorId')??'';
  const controls=window as Window&{
    __formsStrictMount?:(value:boolean)=>void;
    __formsStrictApp?:(value:string)=>void;
    __formsStrictView?:(value:string)=>void;
    __formsStrictDirty?:boolean;
  };
  controls.__formsStrictMount=setMounted;
  controls.__formsStrictApp=setAppId;
  controls.__formsStrictView=setViewId;
  controls.__formsStrictDirty=dirty;
  if(!mounted)return null;
  return query.get('mode')==='structure'?<ApplicationStructurePanel appId={appId} actorId={actorId}
      onDirtyChange={setDirty}/>:
    <FormDesigner appId={appId} viewId={viewId} actorId={actorId} onDirtyChange={setDirty}
      onBack={query.get('back')==='none'?undefined:()=>setMounted(false)}/>;
}
createRoot(document.getElementById('root')!).render(<StrictMode><StrictHarness/></StrictMode>);
