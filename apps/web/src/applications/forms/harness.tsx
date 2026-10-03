import { createRoot } from 'react-dom/client';
import { useState } from 'react';
import { ApplicationStructurePanel } from './ApplicationStructurePanel';
import { FormDesigner } from './FormDesigner';

const query = new URLSearchParams(location.search);
const appId = query.get('appId') ?? '';
const initialViewId = query.get('viewId') ?? '';
function Harness(){
  const [viewId,setViewId]=useState(initialViewId);
  return <>{query.get('switchViewId')&&<button type="button" onClick={()=>setViewId(query.get('switchViewId')!)}>切换视图</button>}
    {query.get('mode') === 'structure'
      ? <ApplicationStructurePanel appId={appId} />
      : <FormDesigner appId={appId} viewId={viewId} />}</>;
}
createRoot(document.getElementById('root')!).render(<Harness/>);
