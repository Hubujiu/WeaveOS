import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { workspaceApi, WorkspaceError } from './workspace-api';
import type { Access, Definition, Department, Member, PageData, Permission, Activity } from './workspace-types';
import { Modal } from './Modal';
import { AdminMaterial } from './AdminMaterial';
import { PersonnelSelect, PersonnelTabs } from './PersonnelControls';
import { Table, type TableColumn, type SortState } from './vendor/arca/components/motion/table';
import './vendor/arca/arca.css';
import './personnel-table.css';
import { TablePresetManager } from './TablePresetManager';
import type {AppliedPreset,TablePreset,PresetOptions} from './TablePresetState';
import { usePersonnelQuery } from './usePersonnelQuery';
import type { ActivityAction, ActivityDisplay, MemberFilterGroup, EventFilterGroup, PersonnelDraft, PersonnelDraftSummary, DraftCreateInput, DraftPayloadByKind } from './query-contracts';
import usersIcon from './assets/admin-users.svg';
import settingsIcon from './assets/admin-settings.svg';
import saveIcon from './assets/admin-save.svg';
import exitIcon from './assets/admin-exit.svg';
import searchIcon from './assets/admin-search.svg';
import plusIcon from './assets/admin-plus.svg';
import plusWhite from './assets/admin-plus-white.svg';

type Tab='成员与部门'|'身份'|'权限模板'|'操作记录';
type Dialog='impact'|'dirty'|'department'|'renameDepartment'|'deleteDepartment'|'invitation'|'member'|'groups'|'delete'|'drafts'|null;
const blankPage=<T,>():PageData<T>=>({items:[],total:0,page:1,pageSize:20});
function toggle(values:string[],id:string){return values.includes(id)?values.filter(v=>v!==id):[...values,id];}
function same(a:Definition|null,b:Definition|null){return JSON.stringify(a)===JSON.stringify(b);}
const activityNames:Record<string,string>={DEPARTMENT_CREATED:'新建部门',DEPARTMENT_UPDATED:'重命名部门',DEPARTMENT_DELETED:'删除部门',IDENTITY_CREATED:'新建身份',IDENTITY_UPDATED:'修改身份',IDENTITY_DELETED:'删除身份',TEMPLATE_CREATED:'新建权限模板',TEMPLATE_UPDATED:'修改权限模板',TEMPLATE_DELETED:'删除权限模板',MEMBER_IDENTITIES_UPDATED:'分配身份',MEMBER_GROUPS_UPDATED:'调整分组',INVITATION_CREATED:'邀请成员'};
export function PersonnelAdmin({access,onDirty,onExit,onUnauthorized}:{access:Access;onDirty:(v:boolean)=>void;onExit:()=>void;onUnauthorized:()=>void}){
 const [tab,setTab]=useState<Tab>('成员与部门');
 const [departments,setDepartments]=useState<Department[]>([]);const [identities,setIdentities]=useState<PageData<Definition>>(blankPage);const [templates,setTemplates]=useState<PageData<Definition>>(blankPage);
 const [permissions,setPermissions]=useState<Permission[]>([]);
 const [selected,setSelected]=useState<Definition|null>(null);const [draft,setDraft]=useState<Definition|null>(null);
 const [dialog,setDialog]=useState<Dialog>(null);const [next,setNext]=useState<(()=>void)|null>(null);
 const [departmentName,setDepartmentName]=useState('');const [departmentId,setDepartmentId]=useState('');const [member,setMember]=useState<Member|null>(null);const [identityIds,setIdentityIds]=useState<string[]>([]);
 const [invitation,setInvitation]=useState('');const [error,setError]=useState('');const [status,setStatus]=useState('');const [pending,setPending]=useState(false);const [loading,setLoading]=useState(true);
 const [memberSearch,setMemberSearch]=useState('');const [memberPage,setMemberPage]=useState(1);
 const [eventSearch,setEventSearch]=useState('');const [eventAction,setEventAction]=useState('');
 const [groupOperation,setGroupOperation]=useState('add');const [groupTarget,setGroupTarget]=useState('');const [groupSource,setGroupSource]=useState('');
 const [allIdentities,setAllIdentities]=useState<Definition[]>([]);const [allTemplates,setAllTemplates]=useState<Definition[]>([]);
 const [departmentFilter,setDepartmentFilter]=useState('');const [identityFilter,setIdentityFilter]=useState('');
 const [definitionSearch,setDefinitionSearch]=useState('');const [definitionPage,setDefinitionPage]=useState(1);
 const [eventPage,setEventPage]=useState(1);const [fromDraft,setFromDraft]=useState('');const [toDraft,setToDraft]=useState('');const [eventRange,setEventRange]=useState({from:'',to:''});
 const [formKind,setFormKind]=useState<Dialog>(null);const [formInitial,setFormInitial]=useState('');const [returnDialog,setReturnDialog]=useState<Dialog>(null);
 const [selectedMembers,setSelectedMembers]=useState<string[]>([]);
 const [memberPageSize,setMemberPageSize]=useState(20);const [eventPageSize,setEventPageSize]=useState(20);
 const [memberFilter,setMemberFilter]=useState<MemberFilterGroup>();const [eventFilter,setEventFilter]=useState<EventFilterGroup>();
 const [memberPreset,setMemberPreset]=useState<AppliedPreset|null>(null),[eventPreset,setEventPreset]=useState<AppliedPreset|null>(null);
 const [memberHidden,setMemberHidden]=useState<string[]>([]),[eventHidden,setEventHidden]=useState<string[]>([]);
 const memberBaseline=useRef<string[]|null>(null),eventBaseline=useRef<string[]|null>(null);
 const [memberPresetDirty,setMemberPresetDirty]=useState(false),[eventPresetDirty,setEventPresetDirty]=useState(false);
 const [eventSort,setEventSort]=useState<SortState|null>({key:'occurredAt',direction:'desc'});
 const [memberWidths,setMemberWidths]=useState<Record<string,number>>({}),[eventWidths,setEventWidths]=useState<Record<string,number>>({});
 const [memberOrder,setMemberOrder]=useState<string[]>([]),[eventOrder,setEventOrder]=useState<string[]>([]);
 const memberParameters={page:memberPage,pageSize:memberPageSize,search:memberSearch,...(departmentFilter?{departmentId:departmentFilter}:{}),...(identityFilter?{identityId:identityFilter}:{})};
 const memberQuery=usePersonnelQuery<Member>('members',{...memberParameters,...(memberFilter?{filter:memberFilter}:{})});
 const eventParameters={page:eventPage,pageSize:eventPageSize,search:eventSearch,...(eventAction?{action:eventAction as ActivityAction}:{}),...(eventRange.from?{from:eventRange.from}:{}),...(eventRange.to?{to:eventRange.to}:{})};
 const eventSortParameters=eventSort?{sortBy:'occurredAt' as const,sortDirection:eventSort.direction}:{};
 const eventQuery=usePersonnelQuery<Activity&{display:ActivityDisplay}>('events',{...eventParameters,...(eventFilter?{filter:eventFilter}:{}),...eventSortParameters});
 const members=memberQuery.data,events=eventQuery.data;
 const [draftList,setDraftList]=useState<PersonnelDraftSummary[]>([]),[savedDraft,setSavedDraft]=useState<PersonnelDraft|null>(null);
 const [draftConflict,setDraftConflict]=useState<{message:string;latest?:Member|Department|Definition;missing:string[]}|null>(null);
 const [draftSaveConflict,setDraftSaveConflict]=useState(false),[departmentVersion,setDepartmentVersion]=useState(1);
 const auxiliarySequence=useRef(0),draftSequence=useRef(0);
 const formValue=formKind==='member'?JSON.stringify([...identityIds].sort()):formKind==='groups'?JSON.stringify([groupOperation,groupTarget,groupSource]):JSON.stringify([departmentName,departmentId]);
 const formDirty=!!formKind&&formValue!==formInitial;const definitionDirty=!same(selected,draft);const dirty=definitionDirty||formDirty||memberPresetDirty||eventPresetDirty;
 const selectedDepartment=departments.find(d=>d.id===departmentFilter);
 function openForm(kind:Dialog,initial:string){setFormKind(kind);setFormInitial(initial);setDialog(kind);setError('');setSavedDraft(null);setDraftConflict(null);setDraftSaveConflict(false);if(kind==='renameDepartment'&&selectedDepartment)setDepartmentVersion(selectedDepartment.version);}
 function clearDialog(){setDialog(null);setFormKind(null);setNext(null);setReturnDialog(null);setInvitation('');setSavedDraft(null);setDraftConflict(null);setDraftSaveConflict(false);}
 function closeDialog(){if(dialog==='dirty'){setDialog(returnDialog);setNext(null);setReturnDialog(null);}else if(formDirty){setReturnDialog(dialog);setNext(()=>clearDialog);setDialog('dirty');}else clearDialog();}
 async function allDefinitions(kind:string){const items:Definition[]=[];let page=1;for(;;){const result=await workspaceApi<PageData<Definition>>('personnel/'+kind+'?pageSize=100&page='+page);items.push(...result.items);if(items.length>=result.total)return items;if(!result.items.length)throw new Error('配置列表未完整加载，请重试');page++;}}
 async function presetOptions():Promise<PresetOptions>{const [ds,ids]=await Promise.all([workspaceApi<{items:Department[]}>('personnel/departments'),allDefinitions('identities')]);setDepartments(ds.items);setAllIdentities(ids);return {departmentIds:ds.items.map(d=>({value:d.id,label:d.name})),identityIds:ids.map(i=>({value:i.id,label:i.name}))};}
 async function applyMemberPreset(preset:TablePreset|null){
  if(memberQuery.blocked)throw memberQuery.error??new WorkspaceError(409,'COMMON_QUERY_CONTEXT_EXPIRED');
  const nextFilter=preset?.filter as MemberFilterGroup|null|undefined;
  const input={...memberParameters,page:1,...(nextFilter?{filter:nextFilter}:{})};
  const changed=memberPage!==1||JSON.stringify(nextFilter??null)!==JSON.stringify(memberFilter??null);
  const receipt=changed?await memberQuery.prepareChange(input):()=>true;if(!receipt)return false;
  if(!receipt())return false;
  if(preset&&!memberPreset)memberBaseline.current=[...memberHidden];
  setMemberFilter(nextFilter??undefined);setMemberHidden(preset?[...preset.hiddenColumnIds]:memberBaseline.current??[]);
  setMemberPreset(preset?{id:preset.id,version:preset.version,filter:preset.filter,hiddenColumnIds:[...preset.hiddenColumnIds]}:null);
  if(!preset)memberBaseline.current=null;setMemberPage(1);setSelectedMembers([]);return true;
 }
 async function applyEventPreset(preset:TablePreset|null){
  if(eventQuery.blocked)throw eventQuery.error??new WorkspaceError(409,'COMMON_QUERY_CONTEXT_EXPIRED');
  const nextFilter=preset?.filter as EventFilterGroup|null|undefined;
  const input={...eventParameters,page:1,...(nextFilter?{filter:nextFilter}:{}),...eventSortParameters};
  const changed=eventPage!==1||JSON.stringify(nextFilter??null)!==JSON.stringify(eventFilter??null);
  const receipt=changed?await eventQuery.prepareChange(input):()=>true;if(!receipt)return false;
  if(!receipt())return false;
  if(preset&&!eventPreset)eventBaseline.current=[...eventHidden];
  setEventFilter(nextFilter??undefined);setEventHidden(preset?[...preset.hiddenColumnIds]:eventBaseline.current??[]);
  setEventPreset(preset?{id:preset.id,version:preset.version,filter:preset.filter,hiddenColumnIds:[...preset.hiddenColumnIds]}:null);
  if(!preset)eventBaseline.current=null;setEventPage(1);return true;
 }
 function pageControls(page:number,data:PageData<unknown>,setPage:(value:number)=>void){return <div className="table-footer"><span>共 {data.total} 项 · 第 {page} 页 / 共 {Math.max(1,Math.ceil(data.total/data.pageSize))} 页</span><button className="text-button" disabled={page<=1} onClick={()=>setPage(page-1)}>上一页</button><button className="text-button" disabled={page*data.pageSize>=data.total} onClick={()=>setPage(page+1)}>下一页</button></div>;}
 useLayoutEffect(()=>{onDirty(dirty);},[dirty,onDirty]);
 useEffect(()=>{setSelectedMembers([]);},[memberSearch,memberPage,memberPageSize,departmentFilter,identityFilter,memberFilter]);
 function fail(e:unknown){if(e instanceof WorkspaceError&&e.status===401){onUnauthorized();return;}if(e instanceof WorkspaceError&&(e.code==='COMMON_QUERY_CHANGED'||e.code==='COMMON_QUERY_CONTEXT_EXPIRED'||e.reason==='QUERY_BUSY')){memberQuery.reject(e);return;}setError(e instanceof Error?e.message:'服务暂时不可用，请稍后重试');}
 async function refreshAux(){
  const sequence=++auxiliarySequence.current;
  setLoading(true);
  try{
   const [ds,ids,ts,ps,allIds,allTs]=await Promise.all([
    workspaceApi<{items:Department[]}>('personnel/departments'),workspaceApi<PageData<Definition>>('personnel/identities?'+new URLSearchParams({page:String(tab==='身份'?definitionPage:1),search:tab==='身份'?definitionSearch:''})),
    workspaceApi<PageData<Definition>>('personnel/templates?'+new URLSearchParams({page:String(tab==='权限模板'?definitionPage:1),search:tab==='权限模板'?definitionSearch:''})),workspaceApi<{items:Permission[]}>('personnel/permissions'),
    allDefinitions('identities'),allDefinitions('templates')]);
   if(sequence!==auxiliarySequence.current)return;
   setDepartments(ds.items);setIdentities(ids);setTemplates(ts);setPermissions(ps.items);setAllIdentities(allIds);setAllTemplates(allTs);
   if(!departmentId)setDepartmentId(ds.items.find(v=>v.isRoot)?.id||'');
  }catch(e){if(sequence===auxiliarySequence.current)fail(e);}finally{if(sequence===auxiliarySequence.current)setLoading(false);}
 }
 async function refresh(){setMemberPage(1);setEventPage(1);setSelectedMembers([]);memberQuery.refresh();eventQuery.refresh();await refreshAux();await loadDrafts();}
 useEffect(()=>{void refreshAux();void loadDrafts();return()=>{auxiliarySequence.current++;draftSequence.current++;};},[]);
 useEffect(()=>{if(memberQuery.error?.status===401||eventQuery.error?.status===401)onUnauthorized();},[memberQuery.error,eventQuery.error,onUnauthorized]);
 useEffect(()=>{if(tab!=='身份'&&tab!=='权限模板')return;let active=true;const query=new URLSearchParams({search:definitionSearch,page:String(definitionPage)});workspaceApi<PageData<Definition>>('personnel/'+(tab==='身份'?'identities':'templates')+'?'+query).then(value=>{if(active)(tab==='身份'?setIdentities:setTemplates)(value);}).catch(e=>{if(active)fail(e);});return()=>{active=false;};},[tab,definitionSearch,definitionPage]);
 function applyEventRange(){const from=fromDraft?new Date(fromDraft):null,to=toDraft?new Date(toDraft):null;if((from&&!Number.isFinite(from.getTime()))||(to&&!Number.isFinite(to.getTime()))||(from&&to&&from>=to)){setError('请填写有效时间，结束时间须晚于开始时间');return;}setError('');setEventPage(1);setEventRange({from:from?.toISOString()||'',to:to?.toISOString()||''});}
 function guarded(action:()=>void){if(dirty){setReturnDialog(dialog);setNext(()=>action);setDialog('dirty');}else action();}
 function changeTab(value:Tab){guarded(()=>{if(value==='成员与部门')memberQuery.recheck();else if(value==='操作记录')eventQuery.recheck();setTab(value);setDefinitionSearch('');setDefinitionPage(1);setSelected(null);setDraft(null);setError('');setStatus('');});}
 function choose(value:Definition){guarded(()=>{setSelected(structuredClone(value));setDraft(structuredClone(value));setSavedDraft(null);setDraftConflict(null);setDraftSaveConflict(false);setError('');setStatus('');});}
 async function save(){
  if(!draft||pending||draftConflict)return;if(!draft.name.trim()){setError('请填写名称');setDialog(null);return;}setPending(true);setError('');
  try{
   const body={name:draft.name,description:draft.description,...(selected?{version:draft.version}:{}),...(tab==='身份'?{templateIds:draft.templateIds||[]}:{}),permissionCodes:draft.permissionCodes,...draftAcknowledgement()};
   const value=await workspaceApi<Definition>('personnel/'+(tab==='身份'?'identities':'templates')+(selected?'/'+draft.id:''),selected?'PUT':'POST',body);
   setSelected(value);setDraft(value);setSavedDraft(null);setDialog(null);await refreshAux();await loadDrafts();setStatus('已保存');
  }catch(e){setDialog(null);fail(e);}finally{setPending(false);}
 }
 async function createDepartment(){
  if(pending||draftConflict||!requireQuery())return;if(!departmentName.trim()){setError('请填写部门名称');return;}setPending(true);setError('');
  try{await workspaceApi('personnel/departments','POST',{name:departmentName,parentId:departmentId||null,queryVersion:members.queryVersion,...draftAcknowledgement()});clearDialog();setDepartmentName('');setStatus('部门已创建');await refresh();}catch(e){fail(e);}finally{setPending(false);}
 }
 async function createInvitation(){
  if(pending||invitation)return;setPending(true);setError('');
  try{const value=await workspaceApi<{id:string;invitationCode:string}>('invitations','POST',{...(members.queryVersion?{queryVersion:members.queryVersion}:{})});setInvitation(value.invitationCode);}catch(e){fail(e);}finally{setPending(false);}
 }
 async function assignIdentities(){
  if(!member||pending||draftConflict||!requireQuery())return;setPending(true);setError('');
  try{await workspaceApi('personnel/members/'+member.id+'/identities','PUT',{identityIds,version:member.version,queryVersion:members.queryVersion,...draftAcknowledgement()});clearDialog();setStatus('身份已分配');await refresh();}catch(e){fail(e);}finally{setPending(false);}
 }
 function newDefinition(){guarded(()=>{setSelected(null);setDraft({id:'',name:'',description:'',version:0,templateIds:[],permissionCodes:[],affectedMembers:0,affectedIdentities:0});setSavedDraft(null);setDraftConflict(null);setDraftSaveConflict(false);setError('');setStatus('');});}
 async function adjustGroups(){
  if(!member||pending||draftConflict||!requireQuery())return;if(!groupTarget||(groupOperation==='move'&&!groupSource)){setError('请选择部门');return;}setPending(true);setError('');
  try{await workspaceApi('personnel/members/'+member.id+'/groups','POST',{operation:groupOperation,departmentId:groupTarget,...(groupOperation==='move'?{sourceDepartmentId:groupSource}:{}),version:member.version,queryVersion:members.queryVersion,...draftAcknowledgement()});clearDialog();await refresh();setStatus('分组已调整');}catch(e){fail(e);}finally{setPending(false);}
 }
 async function deleteDefinition(){
  if(!selected||pending)return;setPending(true);setError('');
  try{await workspaceApi('personnel/'+(tab==='身份'?'identities':'templates')+'/'+selected.id+'?version='+selected.version,'DELETE');setSelected(null);setDraft(null);setDialog(null);await refreshAux();await loadDrafts();setStatus(tab==='身份'?'身份已删除':'模板已删除');}catch(e){setDialog(null);fail(e);}finally{setPending(false);}
 }
 async function saveDepartment(){if(!selectedDepartment||pending||draftConflict||!requireQuery())return;if(!departmentName.trim()){setError('请填写部门名称');return;}setPending(true);setError('');try{await workspaceApi('personnel/departments/'+selectedDepartment.id,'PUT',{name:departmentName,version:departmentVersion,queryVersion:members.queryVersion,...draftAcknowledgement()});clearDialog();await refresh();setStatus('部门已改名');}catch(e){fail(e);}finally{setPending(false);}}
 async function deleteDepartment(){if(!selectedDepartment||pending||!requireQuery())return;setPending(true);setError('');try{await workspaceApi('personnel/departments/'+selectedDepartment.id+'?'+new URLSearchParams({version:String(selectedDepartment.version),queryVersion:members.queryVersion}),'DELETE');setDepartmentFilter('');clearDialog();await refresh();setStatus('部门已删除');}catch(e){clearDialog();fail(e);}finally{setPending(false);}}
 function requireQuery(){if(memberQuery.blocked){memberQuery.reject(new WorkspaceError(409,memberQuery.error?.code||'COMMON_QUERY_CONTEXT_EXPIRED'));return false;}return true;}
 function draftAcknowledgement(){return savedDraft?{draftRef:{id:savedDraft.id,version:savedDraft.version}}:{};}
 async function loadDrafts(){const sequence=++draftSequence.current;try{const value=await workspaceApi<{items:PersonnelDraftSummary[]}>('personnel/drafts');if(sequence===draftSequence.current)setDraftList(value.items);}catch(e){if(sequence===draftSequence.current)fail(e);}}
 function draftInput():DraftCreateInput|null{
  if(formKind==='member'&&member)return {kind:'member-identities',targetId:member.id,baseVersion:member.version,payload:{identityIds}};
  if(formKind==='groups'&&member)return {kind:'member-groups',targetId:member.id,baseVersion:member.version,payload:{operation:groupOperation as DraftPayloadByKind['member-groups']['operation'],departmentId:groupTarget||null,sourceDepartmentId:groupSource||null}};
  if(formKind==='department')return {kind:'department',targetId:null,baseVersion:null,payload:{name:departmentName,parentId:departmentId||null}};
  if(formKind==='renameDepartment')return {kind:'department',targetId:departmentId,baseVersion:departmentVersion,payload:{name:departmentName,parentId:selectedDepartment?.parentId??null}};
  if(draft&&tab==='身份')return {kind:'identity',...(selected?{targetId:draft.id,baseVersion:draft.version}:{targetId:null,baseVersion:null}),payload:{name:draft.name,description:draft.description,templateIds:draft.templateIds||[],permissionCodes:draft.permissionCodes}};
  if(draft&&tab==='权限模板')return {kind:'template',...(selected?{targetId:draft.id,baseVersion:draft.version}:{targetId:null,baseVersion:null}),payload:{name:draft.name,description:draft.description,permissionCodes:draft.permissionCodes}};
  return null;
 }
 async function saveDraft(){const input=draftInput();if(!input||pending)return;setPending(true);setError('');try{const value=await workspaceApi<PersonnelDraft>('personnel/drafts'+(savedDraft?'/'+savedDraft.id:''),savedDraft?'PUT':'POST',savedDraft?{version:savedDraft.version,payload:input.payload}:input);setSavedDraft(value);setDraftSaveConflict(false);await loadDrafts();setStatus('草稿已保存，正式配置未提交');}catch(e){if(e instanceof WorkspaceError&&e.code==='PERSONNEL_DRAFT_CONFLICT')setDraftSaveConflict(true);fail(e);}finally{setPending(false);}}
 async function adoptDraftVersion(){if(!savedDraft||pending)return;setPending(true);try{const latest=await workspaceApi<PersonnelDraft>('personnel/drafts/'+savedDraft.id);setSavedDraft(latest);setDraftSaveConflict(false);setError('');setStatus('已采用最新草稿版本，当前输入仍保留；请点击保存草稿确认覆盖');}catch(e){fail(e);}finally{setPending(false);}}
 function invalidReferences(input:DraftCreateInput,ds:Department[],ids:Definition[],ts:Definition[],ps:Permission[]):string[]{
  const has=(items:{id:string}[],id:string|null)=>!id||items.some(item=>item.id===id);
  if(input.kind==='member-identities')return input.payload.identityIds.filter(id=>!has(ids,id)).map(id=>'身份 '+id);
  if(input.kind==='member-groups')return [input.payload.departmentId,input.payload.sourceDepartmentId].filter(id=>!has(ds,id)).map(id=>'部门 '+id);
  if(input.kind==='department')return has(ds,input.payload.parentId)?[]:['父部门 '+input.payload.parentId];
  const missing=input.payload.permissionCodes.filter(code=>!ps.some(p=>p.code===code)).map(code=>'权限 '+code);
  if(input.kind==='identity')missing.push(...input.payload.templateIds.filter(id=>!has(ts,id)).map(id=>'模板 '+id));
  return missing;
 }
 async function restoreDraft(summary:PersonnelDraftSummary){if(pending)return;setPending(true);setError('');try{
  const stored=await workspaceApi<PersonnelDraft>('personnel/drafts/'+summary.id);
  const [ds,ids,ts,ps]=await Promise.all([workspaceApi<{items:Department[]}>('personnel/departments'),allDefinitions('identities'),allDefinitions('templates'),workspaceApi<{items:Permission[]}>('personnel/permissions')]);
  setDepartments(ds.items);setAllIdentities(ids);setAllTemplates(ts);setPermissions(ps.items);
  let latest:Member|Department|Definition|undefined;
  if(stored.targetId){try{latest=stored.kind.startsWith('member-')?await workspaceApi<Member>('personnel/members/'+stored.targetId):stored.kind==='department'?ds.items.find(d=>d.id===stored.targetId):await workspaceApi<Definition>('personnel/'+(stored.kind==='identity'?'identities':'templates')+'/'+stored.targetId);}catch(e){if(!(e instanceof WorkspaceError&&e.status===404))throw e;}}
  const missing=invalidReferences(stored,ds.items,ids,ts,ps.items);
  const changed=!!stored.targetId&&(!latest||latest.version!==stored.baseVersion);
  if(stored.kind==='member-identities'||stored.kind==='member-groups'){
   setTab('成员与部门');setSelected(null);setDraft(null);
   const current=latest as Member|undefined;setMember(current?{...current,version:stored.baseVersion??0}:{...access.user,id:stored.targetId,status:'active',bootstrapAdmin:false,version:stored.baseVersion??0,departmentIds:[],identityIds:[],departments:[],identities:[],permissions:[]});
   if(stored.kind==='member-identities'){setIdentityIds(stored.payload.identityIds);openForm('member',JSON.stringify(current?.identityIds||[]));}
   else {setGroupOperation(stored.payload.operation);setGroupTarget(stored.payload.departmentId||'');setGroupSource(stored.payload.sourceDepartmentId||'');openForm('groups',JSON.stringify(['add','','']));}
  }else if(stored.kind==='department'){
   setTab('成员与部门');setSelected(null);setDraft(null);setDepartmentName(stored.payload.name);setDepartmentId(stored.targetId||stored.payload.parentId||'');if(stored.targetId)setDepartmentFilter(stored.targetId);openForm(stored.targetId?'renameDepartment':'department',JSON.stringify([latest&&'name'in latest?latest.name:'',stored.targetId||'']));setDepartmentVersion(stored.baseVersion??1);
  }else{
   setTab(stored.kind==='identity'?'身份':'权限模板');setFormKind(null);setDialog(null);
   const current=latest as Definition|undefined;const base=current||{id:stored.targetId||'',name:'',description:'',version:stored.baseVersion??0,templateIds:[],permissionCodes:[],affectedMembers:0,affectedIdentities:0};
   setSelected(stored.targetId?base:null);setDraft({...base,...stored.payload,version:stored.baseVersion??0});
  }
  setSavedDraft(stored);setDraftSaveConflict(false);
  setDraftConflict(changed||missing.length?{message:!latest&&stored.targetId?'目标已删除，输入保留，不能自动重建':`原版本 ${stored.baseVersion??'新建'}，最新版本 ${latest?.version??'新建'}。请比较差异并明确解决。`,latest,missing}:null);
  setStatus('草稿已恢复，尚未提交正式配置');
 }catch(e){fail(e);}finally{setPending(false);}}
 function resolveDraftConflict(){const input=draftInput();if(!draftConflict||!input)return;const missing=invalidReferences(input,departments,allIdentities,allTemplates,permissions);if(missing.length){setDraftConflict({...draftConflict,missing});return;}if(input.targetId&&!draftConflict.latest)return;const version=draftConflict.latest?.version;if(version!==undefined){if(input.kind.startsWith('member-'))setMember(value=>value?{...value,version}:value);else if(input.kind==='department')setDepartmentVersion(version);else setDraft(value=>value?{...value,version}:value);}setDraftConflict(null);setError('');setStatus('已明确采用最新对象版本；当前输入保留，尚未提交');}
 async function removeDraft(value:PersonnelDraftSummary){if(pending)return;setPending(true);setError('');try{await workspaceApi('personnel/drafts/'+value.id+'?version='+value.version,'DELETE');await loadDrafts();}catch(e){fail(e);}finally{setPending(false);}}
 function draftControls(){return <section className="personnel-draft-controls"><button className="admin-button" disabled={pending} onClick={()=>void refresh()}>刷新查询</button><button className="admin-button" disabled={pending} onClick={()=>void saveDraft()}>保存草稿</button>{savedDraft&&<span>已保存草稿 · 版本 {savedDraft.version}</span>}{draftSaveConflict&&<button className="admin-button" disabled={pending} onClick={()=>void adoptDraftVersion()}>保留当前输入，采用最新草稿版本</button>}{draftConflict&&<div className="draft-conflict" role="alert"><p>{draftConflict.message}</p>{draftConflict.latest&&<details><summary>查看最新目标与当前输入差异</summary><pre>{JSON.stringify({latest:draftConflict.latest,current:draftInput()?.payload},null,2)}</pre></details>}{draftConflict.missing.length>0&&<p>已失效引用：{draftConflict.missing.join('、')}。请取消失效项或重新选择。</p>}<button className="admin-button" disabled={pending||(!draftConflict.latest&&!!savedDraft?.targetId)} onClick={resolveDraftConflict}>保留当前输入，使用最新对象版本</button></div>}</section>;}
 const definition=tab==='身份'||tab==='权限模板';
 const usingTemplates=allTemplates.filter(t=>draft?.templateIds?.includes(t.id));
 const direct=new Set(draft?.permissionCodes||[]);const effective=permissions.filter(p=>direct.has(p.code)||usingTemplates.some(t=>t.permissionCodes.includes(p.code)));
 const memberColumns:TableColumn<Member>[]=[
  {key:'account',header:'成员',dataType:'text',align:'left',cell:m=><div className="table-cell member-name" title={m.account}><strong>{m.account}</strong><small>{m.status==='active'?'正常':'已停用'}</small></div>},
  {key:'departments',header:'部门',width:'176px',dataType:'text',align:'left',cell:m=><div className="table-cell">{m.departments.map(d=>d.name).join('、')||'未分组'}</div>},
  {key:'identities',header:'身份',width:'216px',dataType:'text',align:'left',cell:m=><div className="table-cell">{m.bootstrapAdmin?'Bootstrap Admin':m.identities.map(i=>i.name).join('、')||'未分配'}</div>},
  {key:'personnelManage',header:'人员管理',width:'136px',dataType:'boolean',align:'left',cell:m=><div className="table-cell">{m.bootstrapAdmin?'默认拥有':m.permissions.some(p=>p.code==='personnel.manage')?'已开启':'未开启'}</div>},
  {key:'actions',header:'操作',width:'192px',dataType:'action',align:'left',cell:m=><div className="table-cell"><button className="text-button" disabled={memberQuery.blocked} onClick={()=>{setMember(m);setIdentityIds([...m.identityIds]);openForm('member',JSON.stringify([...m.identityIds].sort()));}}>配置身份</button><button className="text-button" disabled={memberQuery.blocked} onClick={()=>{setMember(m);setGroupOperation('add');setGroupTarget('');setGroupSource(m.departmentIds[0]||'');openForm('groups',JSON.stringify(['add','',m.departmentIds[0]||'']));}}>调整分组</button></div>}
 ];
 const eventColumns:TableColumn<Activity&{display:ActivityDisplay}>[]=[
  {key:'occurredAt',header:'时间',width:'196px',dataType:'time',align:'left',cell:event=><div className="table-cell">{new Date(event.occurredAt).toLocaleString('zh-CN')}</div>},
  {key:'actorAccount',header:'操作者',width:'160px',dataType:'text',align:'left',cell:event=><div className="table-cell">{event.actorAccount}</div>},
  {key:'action',header:'操作',width:'176px',dataType:'enum',align:'left',cell:event=><div className="table-cell">{event.display.action}</div>},
  {key:'object',header:'对象',width:'224px',dataType:'text',align:'left',cell:event=><div className="table-cell">{event.display.object}</div>},
  {key:'detail',header:'变更内容',dataType:'text',align:'left',cell:event=><div className="table-cell">{event.display.detail}</div>},
  {key:'outcome',header:'结果',width:'160px',dataType:'enum',align:'left',cell:event=><div className="table-cell">{event.display.outcome}</div>}
 ];
 const queryError=tab==='操作记录'?eventQuery.error:memberQuery.error;
 return <div className="admin-shell">
  <AdminMaterial/>
  <header className="admin-header"><div className="admin-corner"><img className="admin-settings-icon" src={settingsIcon} width="20" height="20" alt=""/><strong>管理后台</strong></div>
   <div className="admin-actions"><button className="admin-action" disabled={pending} onClick={()=>guarded(()=>{setDialog('drafts');void loadDrafts();})}>草稿箱</button><button className="admin-action save" disabled={!definitionDirty||pending||!!draftConflict} onClick={()=>setDialog('impact')}><img src={saveIcon} width="16" height="16" alt=""/>保存</button><button className="admin-action" onClick={onExit} disabled={pending}><img src={exitIcon} width="16" height="16" alt=""/>退出</button></div>
  </header>
  <aside className="admin-sidebar"><span className="sidebar-caption">管理功能</span><button className="personnel-nav" aria-label="人员管理" onClick={()=>guarded(()=>{setTab('成员与部门');setDraft(null);setSelected(null);})}><img className="personnel-nav-icon" src={usersIcon} width="20" height="20" alt=""/><span className="personnel-nav-label">人员管理</span></button><p className="sidebar-caption">成员 · 部门 · 身份 · 权限</p>
  </aside>
  <main className="personnel-content">
   <div className="personnel-page-heading"><h1>人员管理</h1><p>统一管理成员、部门、身份与应用访问。</p></div>
   <PersonnelTabs value={tab} values={['成员与部门','身份','权限模板','操作记录']} onChange={changeTab}/>
   <div className="query-refresh-row"><button className="admin-button" disabled={pending} onClick={()=>void refresh()}>刷新查询</button>{(loading||memberQuery.loading||eventQuery.loading)&&<span role="status">正在加载…</span>}</div>{queryError&&<p className="workspace-error" role="alert">{queryError.message}</p>}{error&&<p className="workspace-error" role="alert">{error}<button className="text-button" onClick={()=>{setError('');void refresh();}}>重新加载</button></p>}{status&&<p className="workspace-status" role="status">{status}</p>}
   <section key={tab} id="personnel-panel" role="tabpanel" aria-label={tab} className="personnel-panel">
    <div className="section-heading"><div><h2>{tab}</h2><p>{tab==='成员与部门'?'管理成员归属与身份，部门调整不改变访问权限':tab==='身份'?'先配置身份，再把身份分配给成员':tab==='权限模板'?'集中配置中央权限，供多个身份复用':'查看成员、部门与权限配置的变更记录'}</p></div>{tab==='操作记录'&&<PersonnelSelect className="activity-action-filter" label="操作类型" value={eventAction} onChange={value=>{setEventAction(value);setEventPage(1);}} options={[{value:"",label:"全部操作"},...Object.entries(activityNames).map(([value,label])=>({value,label}))]}/>}{definition&&<button className="admin-button primary" onClick={newDefinition}><img src={plusWhite} width="16" height="16" alt=""/>{tab==='身份'?'新建身份':'新建权限模板'}</button>}{tab==='成员与部门'&&<><button className="admin-button" onClick={()=>{setDepartmentName('');openForm('department',JSON.stringify(['',departmentId]));}}><img src={plusIcon} width="16" height="16" alt=""/>新建部门</button><button className="admin-button primary" onClick={()=>{setInvitation('');setDialog('invitation');}}><img src={plusWhite} width="16" height="16" alt=""/>邀请成员</button></>}</div>
    {tab==='成员与部门'?<div className="personnel-workspace">
     <section className="department-panel surface"><h3>部门 <span className="badge">{departments.length}</span></h3><button className={'department-row'+(!departmentFilter?' selected':'')} onClick={()=>{setDepartmentFilter('');setMemberPage(1);}}>全部成员 <small>{members.total}</small></button>{departments.map(d=><button key={d.id} className={'department-row'+(departmentFilter===d.id?' selected':'')} style={{paddingLeft:d.parentId?28:12}} onClick={()=>{setDepartmentId(d.id);setDepartmentFilter(d.id);setMemberPage(1);}}>{d.name}<small>{d.memberCount}</small></button>)}<div className="department-actions"><button className="text-button" disabled={!selectedDepartment} onClick={()=>{if(selectedDepartment){setDepartmentName(selectedDepartment.name);setDepartmentId(selectedDepartment.id);openForm('renameDepartment',JSON.stringify([selectedDepartment.name,selectedDepartment.id]));}}}>重命名部门</button><button className="text-button danger" disabled={!selectedDepartment||selectedDepartment.isRoot||selectedDepartment.memberCount>0||selectedDepartment.childrenCount>0} onClick={()=>setDialog('deleteDepartment')}>删除部门</button></div><p className="panel-note">部门只用于分组。<br/>移动成员不会自动改变<br/>身份或应用访问权限。</p></section>
     <section className="member-table surface"><div className="table-toolbar"><div className="admin-search"><img src={searchIcon} width="16" height="16" alt=""/><input aria-label="搜索成员" placeholder="搜索成员" value={memberSearch} onChange={e=>{setMemberSearch(e.target.value);setMemberPage(1);}}/></div><TablePresetManager view="members" active={memberPreset} hiddenColumnIds={memberHidden} onApply={applyMemberPreset} loadOptions={presetOptions} onDirty={setMemberPresetDirty} onUnauthorized={onUnauthorized} options={{departmentIds:departments.map(d=>({value:d.id,label:d.name})),identityIds:allIdentities.map(i=>({value:i.id,label:i.name}))}}/><span>{members.total} 位成员</span><PersonnelSelect label="筛选身份" value={identityFilter} onChange={value=>{setIdentityFilter(value);setMemberPage(1);}} options={[{value:"",label:"全部身份"},...allIdentities.map(i=>({value:i.id,label:i.name}))]}/></div><div className="arca-source personnel-source-scope"><Table className="personnel-source-table rounded-xl" ariaLabel="成员" data={members.items} columns={memberColumns} hiddenColumnIds={memberHidden} getRowId={row=>row.id} minWidth={1008} rowHeight={40} fillViewport paginated manualPagination manualSorting recordCount={members.total} page={memberQuery.error?members.page:memberPage} pageSize={memberPageSize} pageSizes={[5,10,20,25,50,100]} onPageChange={setMemberPage} onPageSizeChange={size=>{setMemberPageSize(size);setMemberPage(1);}} resizable columnWidths={memberWidths} onColumnWidthsChange={setMemberWidths} reorderable columnOrder={memberOrder} onColumnOrderChange={setMemberOrder} selectable selectedRowIds={selectedMembers} onSelectionChange={setSelectedMembers} selectAllLabel="选择当前页成员" getRowLabel={m=>'选择成员：'+m.account} loading={memberQuery.loading} emptyState="暂无成员"/></div></section>
    </div>:definition?<div className="personnel-workspace">
     <section className="definition-list surface"><div className="admin-search"><img src={searchIcon} width="16" height="16" alt=""/><input aria-label={tab==='身份'?'搜索身份':'搜索权限模板'} placeholder={tab==='身份'?'搜索身份':'搜索权限模板'} value={definitionSearch} onChange={e=>{setDefinitionSearch(e.target.value);setDefinitionPage(1);}}/></div>{(tab==='身份'?identities:templates).items.map(item=><button key={item.id} aria-label={item.name} className={'definition-item'+(draft?.id===item.id?' selected':'')} onClick={()=>choose(item)}><strong>{item.name}</strong><span>{item.description||'暂无说明'}</span><small>{tab==='身份'?item.affectedMembers+' 位成员使用':item.affectedIdentities+' 个身份引用'} · {item.permissionCodes.length} 项直接权限</small></button>)}{!(tab==='身份'?identities:templates).total&&<p className="empty-state">{tab==='身份'?'暂无身份':'暂无权限模板'}</p>}{pageControls(definitionPage,tab==='身份'?identities:templates,setDefinitionPage)}<p className="panel-note">{tab==='身份'?<>身份是可分配给成员的模板。<br/>同一个身份可供多位成员使用。</>:<>一个权限模板可供多个身份复用。<br/>共享修改会影响所有引用者。</>}</p></section>
     {draft?<section className="definition-details surface"><div className="detail-heading"><h2>{selected?.name||(tab==='身份'?'新建身份':'新建权限模板')}</h2><span className="badge">{tab==='身份'?draft.affectedMembers+' 位成员使用':draft.affectedIdentities+' 个身份正在引用'}</span></div><div className="basic-information"><label>{tab==='身份'?'身份名称':'模板名称'}<input aria-label={tab==='身份'?'身份名称':'模板名称'} value={draft.name} maxLength={100} onChange={e=>setDraft({...draft,name:e.target.value})}/></label><label>说明<input aria-label="说明" value={draft.description} maxLength={1000} onChange={e=>setDraft({...draft,description:e.target.value})}/></label></div>
      {tab==='身份'&&<section className="config-section"><h3>权限模板</h3>{(draft.templateIds||[]).filter(id=>!allTemplates.some(t=>t.id===id)).map(id=><label key={id} className="template-choice"><input type="checkbox" aria-label={'已失效模板：'+id} checked onChange={()=>setDraft({...draft,templateIds:toggle(draft.templateIds||[],id)})}/>已失效模板：{id}</label>)}{allTemplates.map(t=><label key={t.id} className="template-choice"><input type="checkbox" aria-label={'模板：'+t.name} checked={draft.templateIds?.includes(t.id)||false} onChange={()=>setDraft({...draft,templateIds:toggle(draft.templateIds||[],t.id)})}/><strong>{t.name}</strong><small>{t.permissionCodes.length} 项权限</small></label>)}{!allTemplates.length&&<p>暂无权限模板</p>}</section>}
      <section className="config-section"><h3>{tab==='身份'?'直接权限':'权限配置'}</h3><p>{tab==='身份'?'可直接选择单项权限，与权限模板合并生效。':'系统管理'}</p>{draft.permissionCodes.filter(code=>!permissions.some(p=>p.code===code)).map(code=><label key={code} className="template-choice"><input type="checkbox" aria-label={'已失效权限：'+code} checked onChange={()=>setDraft({...draft,permissionCodes:toggle(draft.permissionCodes,code)})}/>已失效权限：{code}</label>)}{permissions.filter(p=>p.category==='system').map(p=><label key={p.code} className="permission-item"><input type="checkbox" aria-label={(tab==='身份'?'直接权限：':'中央权限：')+p.name} checked={direct.has(p.code)} onChange={()=>setDraft({...draft,permissionCodes:toggle(draft.permissionCodes,p.code)})}/><span><strong>{p.name}</strong><span>邀请成员、管理部门、身份与权限模板</span>{tab==='权限模板'&&<small>{p.code}</small>}{usingTemplates.filter(t=>t.permissionCodes.includes(p.code)).map(t=><small key={t.id}>来自模板：{t.name}</small>)}</span></label>)}</section>
      {tab==='身份'&&<><div className="effective-permissions"><strong>有效权限</strong>{effective.map(p=><span key={p.code} className="badge">{p.name}</span>)}</div><p className="identity-root-note">人员管理可授予普通成员；它不是 Root 身份。</p></>}
      {(tab==='权限模板'||permissions.some(p=>p.category==='application'))&&<section className="config-section"><h3>应用访问</h3>{permissions.some(p=>p.category==='application')?permissions.filter(p=>p.category==='application').map(p=><label key={p.code} className="permission-item"><input aria-label={(tab==='身份'?'直接权限：':'中央权限：')+p.name} type="checkbox" checked={direct.has(p.code)} onChange={()=>setDraft({...draft,permissionCodes:toggle(draft.permissionCodes,p.code)})}/>{p.name}</label>):<div className="no-applications"><strong>尚未接入业务应用</strong><p>接入知识库或自建应用后，可在此配置使用权限。</p><small>文档、分类和表单的内部权限，由各应用自己管理。</small></div>}</section>}
      {draftControls()}<footer className="definition-footer"><p className="panel-note">配置完成后点击右上角「保存」，将同步影响所有引用者。</p>{<button className="text-button danger" disabled={!selected||dirty||(tab==='身份'?draft.affectedMembers>0:draft.affectedIdentities>0)} onClick={()=>setDialog('delete')}>{tab==='身份'?'删除身份':'删除模板'}</button>}</footer>
     </section>:<section className="definition-details surface empty-state">请选择{tab==='身份'?'身份':'权限模板'}查看配置</section>}
    </div>:<section className="activity-table surface"><div className="table-toolbar"><div className="admin-search"><img src={searchIcon} width="16" height="16" alt=""/><input aria-label="搜索操作记录" placeholder="搜索成员或操作对象" value={eventSearch} onChange={e=>{setEventSearch(e.target.value);setEventPage(1);}}/></div><TablePresetManager view="events" active={eventPreset} hiddenColumnIds={eventHidden} onApply={applyEventPreset} loadOptions={async()=>({})} onDirty={setEventPresetDirty} onUnauthorized={onUnauthorized}/><p className="activity-note">记录仅包含配置变更，不展示密码或邀请码明文。</p><details className="event-time-filter"><summary>{eventRange.from||eventRange.to?'自定义时间':'最近 7 天'}</summary><label>开始时间<input type="datetime-local" aria-label="开始时间" value={fromDraft} onChange={e=>setFromDraft(e.target.value)}/></label><label>结束时间<input type="datetime-local" aria-label="结束时间" value={toDraft} onChange={e=>setToDraft(e.target.value)}/></label><button className="admin-button" onClick={applyEventRange}>应用时间范围</button></details></div><div className="arca-source personnel-source-scope"><Table className="personnel-source-table rounded-xl" ariaLabel="操作记录" data={events.items} columns={eventColumns} hiddenColumnIds={eventHidden} getRowId={row=>row.id} minWidth={1080} rowHeight={40} fillViewport paginated manualPagination manualSorting recordCount={events.total} page={eventQuery.error?events.page:eventPage} pageSize={eventPageSize} pageSizes={[5,10,20,25,50,100]} onPageChange={setEventPage} onPageSizeChange={size=>{setEventPageSize(size);setEventPage(1);}} sort={eventSort} onSortChange={value=>{setEventSort(value);setEventPage(1);}} resizable columnWidths={eventWidths} onColumnWidthsChange={setEventWidths} reorderable columnOrder={eventOrder} onColumnOrderChange={setEventOrder} loading={eventQuery.loading} emptyState="暂无操作记录"/></div></section>}
   </section>
  </main>
  {dialog&&<Modal title={dialog==='drafts'?'草稿箱':dialog==='dirty'?'有未保存的修改':dialog==='impact'?'确认共享修改':dialog==='department'?'新建部门':dialog==='renameDepartment'?'重命名部门':dialog==='deleteDepartment'?'删除部门':dialog==='invitation'?'邀请成员':dialog==='groups'?'调整成员分组':dialog==='delete'?(tab==='身份'?'删除身份':'删除权限模板'):'配置成员身份'} busy={pending} onClose={closeDialog}>
   {dialog==='drafts'?<><p>仅显式保存的草稿可恢复，草稿不改变正式配置。</p>{draftList.length?draftList.map(item=><article className="personnel-draft-item" key={item.id}><strong>{{'member-identities':'成员身份','member-groups':'成员分组',department:'部门',identity:'身份',template:'权限模板'}[item.kind]}</strong><span>版本 {item.version} · {new Date(item.updatedAt).toLocaleString('zh-CN')}</span><button className="admin-button" disabled={pending} onClick={()=>void restoreDraft(item)}>恢复草稿</button><button className="text-button danger" disabled={pending} onClick={()=>void removeDraft(item)}>删除草稿</button></article>):<p>暂无已保存草稿</p>}</>:dialog==='groups'?<><p>{member?.account}：部门调整不会改变身份或应用访问。</p><label>分组操作<PersonnelSelect label="分组操作" value={groupOperation} onChange={setGroupOperation} options={[{value:"add",label:"添加"},{value:"remove",label:"移除"},{value:"move",label:"移动"}]}/></label>{groupOperation==='move'&&<label>来源部门<PersonnelSelect label="来源部门" value={groupSource} onChange={setGroupSource} options={[{value:"",label:"请选择"},...departments.filter(d=>member?.departmentIds.includes(d.id)).map(d=>({value:d.id,label:d.name}))]}/></label>}<label>目标部门<PersonnelSelect label="目标部门" value={groupTarget} onChange={setGroupTarget} options={[{value:"",label:"请选择"},...departments.map(d=>({value:d.id,label:d.name}))]}/></label><div className="dialog-actions"><button className="admin-button primary" disabled={pending} onClick={()=>void adjustGroups()}>确认调整</button></div></>:dialog==='delete'?<><p>确认删除“{selected?.name}”？存在引用时无法删除。</p><div className="dialog-actions"><button className="admin-button" onClick={()=>setDialog(null)}>取消</button><button className="admin-button primary" disabled={pending} onClick={()=>void deleteDefinition()}>确认删除</button></div></>:dialog==='dirty'?<><p>切换将丢失未保存的修改。</p><div className="dialog-actions"><button className="admin-button" onClick={()=>{setDialog(returnDialog);setNext(null);setReturnDialog(null);}}>继续编辑</button><button className="admin-button primary" onClick={()=>{setDialog(null);setFormKind(null);setDraft(selected);next?.();setNext(null);setReturnDialog(null);}}>放弃修改</button></div></>:dialog==='impact'?<><p>本次修改将同步影响 {draft?.affectedMembers||0} 位成员{tab==='权限模板'?'，以及 '+(draft?.affectedIdentities||0)+' 个身份':''}。</p><div className="dialog-actions"><button className="admin-button" onClick={()=>setDialog(null)}>继续编辑</button><button className="admin-button primary" disabled={pending} onClick={()=>void save()}>确认保存</button></div></>:dialog==='deleteDepartment'?<><p>确认删除“{selectedDepartment?.name}”？部门必须没有成员或子部门。</p><div className="dialog-actions"><button className="admin-button" onClick={clearDialog}>取消</button><button className="admin-button primary" disabled={pending} onClick={()=>void deleteDepartment()}>确认删除</button></div></>:dialog==='department'||dialog==='renameDepartment'?<><label>部门名称<input aria-label="部门名称" value={departmentName} maxLength={100} onChange={e=>setDepartmentName(e.target.value)}/></label>{dialog==='department'&&<label>父部门<PersonnelSelect label="父部门" value={departmentId} onChange={setDepartmentId} options={departments.map(d=>({value:d.id,label:d.name}))}/></label>}<div className="dialog-actions"><button className="admin-button primary" disabled={pending} onClick={()=>void (dialog==='department'?createDepartment():saveDepartment())}>{dialog==='department'?'确认创建':'确认重命名'}</button></div></>:dialog==='invitation'?<><p>生成单次邀请码，注册成功后不可再次使用。请安全转交给受邀成员。</p>{invitation?<label>邀请码<input aria-label="邀请码" readOnly value={invitation}/></label>:<button className="admin-button primary" disabled={pending} onClick={()=>void createInvitation()}>生成邀请码</button>}</>:<><p>{member?.account}</p>{identityIds.filter(id=>!allIdentities.some(i=>i.id===id)).map(id=><label key={id} className="template-choice"><input type="checkbox" aria-label={'已失效身份：'+id} checked onChange={()=>setIdentityIds(toggle(identityIds,id))}/>已失效身份：{id}</label>)}{allIdentities.map(i=><label key={i.id} className="template-choice"><input type="checkbox" aria-label={'身份：'+i.name} checked={identityIds.includes(i.id)} onChange={()=>setIdentityIds(toggle(identityIds,i.id))}/>{i.name}</label>)}{!allIdentities.length&&<p>暂无可分配身份</p>}<div className="dialog-actions"><button className="admin-button primary" disabled={pending} onClick={()=>void assignIdentities()}>确认分配</button></div></>}
   {formKind&&dialog===formKind&&draftControls()}{error&&<p className="workspace-error" role="alert">{error}</p>}{memberQuery.error&&formKind&&<p className="workspace-error" role="alert">{memberQuery.error.message}</p>}
  </Modal>}
 </div>;
}
