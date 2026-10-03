import {createRoot} from 'react-dom/client';
import {RecordWorkspace} from './RecordWorkspace';
const events={creates:[] as unknown[],opened:[] as unknown[],unauthorized:0,mismatch:0};
Object.assign(window,{__rootRecordWorkspace:events});
createRoot(document.getElementById('root')!).render(<RecordWorkspace
 actorId="11111111-1111-4111-8111-111111111111"
 appId="22222222-2222-4222-8222-222222222222"
 viewId="33333333-3333-4333-8333-333333333333"
 onCreate={(view,queryVersion)=>events.creates.push({view,queryVersion})}
 onOpenRecord={(view,record,queryVersion)=>events.opened.push({view,record,queryVersion})}
 onUnauthorized={()=>{events.unauthorized++;}} onIdentityMismatch={()=>{events.mismatch++;}}/>);
