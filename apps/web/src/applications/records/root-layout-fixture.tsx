import {createRoot} from 'react-dom/client';
import {RecordForm} from './RecordForm';
import type {RecordItem,RuntimeField,RuntimeView} from './contracts';
const actorId='11111111-1111-4111-8111-111111111111',appId='22222222-2222-4222-8222-222222222222',viewId='33333333-3333-4333-8333-333333333333';
const title='44444444-4444-4444-8444-444444444444',amount='55555555-5555-4555-8555-555555555555',hidden='66666666-6666-4666-8666-666666666666',denied='99999999-9999-4999-8999-999999999999';
const field=(id:string,name:string,kind:RuntimeField['kind'],create=true):RuntimeField=>({id,name,kind,required:false,presentation:{helpText:null,displayTimeZone:null},input:{},access:{create,read:create?'all':'none',edit:create?'all':'none',history:'none'},query:{operators:['eq','neq'],sortable:false,quickSearchable:false}});
const view:RuntimeView={appId,tableId:appId,viewId,schemaVersion:1,viewVersion:1,policyRevision:1,
 fields:[field(title,'事由','text'),field(amount,'金额','money'),field(hidden,'未放入布局','text'),field(denied,'无权字段','text',false)],
 layout:[{id:'aaaaaaaa-aaaa-4aaa-8aaa-000000000001',kind:'group',title:'基本信息',children:[{id:'aaaaaaaa-aaaa-4aaa-8aaa-000000000002',kind:'field',fieldId:amount,span:6},{id:'aaaaaaaa-aaaa-4aaa-8aaa-000000000003',kind:'field',fieldId:title,span:6},{id:'aaaaaaaa-aaaa-4aaa-8aaa-000000000004',kind:'field',fieldId:denied}]},{id:'aaaaaaaa-aaaa-4aaa-8aaa-000000000005',kind:'divider'},{id:'aaaaaaaa-aaaa-4aaa-8aaa-000000000006',kind:'system_field',fieldId:'recordVersion'}],
 capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}};
const read=new URLSearchParams(location.search).get('mode')==='read';
const record:RecordItem={id:'77777777-7777-4777-8777-777777777777',appId,tableId:appId,viewId,createdBy:actorId,createdAt:'2026-10-03T09:00:00Z',updatedAt:'2026-10-03T09:01:00Z',recordVersion:4,schemaVersion:1,values:{[title]:'差旅报销',[amount]:'9007199254740993.01',[hidden]:'隐藏内容',[denied]:'无权内容'},referenceDisplays:{}};
createRoot(document.getElementById('root')!).render(<RecordForm view={view}
 identity={read?{kind:'record',actorId,appId,viewId,recordId:record.id}:{kind:'new',actorId,appId,viewId,clientDraftId:'88888888-8888-4888-8888-888888888888'}}
 record={read?record:undefined} mode={read?'read':'create'} authorityKey="layout-root-1"
 onUnauthorized={()=>{}} onIdentityMismatch={()=>{}} registerLeaveGuard={()=>()=>{}} onRefresh={()=>{}}
 onConfirmed={()=>{throw new Error('layout test must not save');}} onDirtyChange={()=>{}} onRequestDiscard={()=>{}} onDiscard={()=>{}}/>);
