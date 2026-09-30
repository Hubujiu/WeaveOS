import { useEffect, useLayoutEffect, useState } from 'react';
import { workspaceApi, WorkspaceError } from './workspace-api';
import type { Access, Definition, Department, Member, PageData, Permission, Activity } from './workspace-types';
import { Modal } from './Modal';
import usersIcon from './assets/admin-users.svg';
import settingsIcon from './assets/admin-settings.svg';
import grip from './assets/nav-grip.svg';
import chevron from './assets/admin-chevron.svg';
import saveIcon from './assets/admin-save.svg';
import exitIcon from './assets/admin-exit.svg';
import searchIcon from './assets/admin-search.svg';
import plusIcon from './assets/admin-plus.svg';
import plusWhite from './assets/admin-plus-white.svg';

type Tab='成员与部门'|'身份'|'权限模板'|'操作记录';
type Dialog='impact'|'dirty'|'department'|'invitation'|'member'|'groups'|'delete'|null;
const blankPage=<T,>():PageData<T>=>({items:[],total:0,page:1,pageSize:20});
function toggle(values:string[],id:string){return values.includes(id)?values.filter(v=>v!==id):[...values,id];}
function same(a:Definition|null,b:Definition|null){return JSON.stringify(a)===JSON.stringify(b);}
export function PersonnelAdmin({access,onDirty,onExit,onUnauthorized}:{access:Access;onDirty:(v:boolean)=>void;onExit:()=>void;onUnauthorized:()=>void}){
 const [tab,setTab]=useState<Tab>('成员与部门');const [top,setTop]=useState(true);const [side,setSide]=useState(()=>innerWidth>=900);
 const [departments,setDepartments]=useState<Department[]>([]);const [identities,setIdentities]=useState<PageData<Definition>>(blankPage);const [templates,setTemplates]=useState<PageData<Definition>>(blankPage);
 const [members,setMembers]=useState<PageData<Member>>(blankPage);const [events,setEvents]=useState<PageData<Activity>>(blankPage);const [permissions,setPermissions]=useState<Permission[]>([]);
 const [selected,setSelected]=useState<Definition|null>(null);const [draft,setDraft]=useState<Definition|null>(null);
 const [dialog,setDialog]=useState<Dialog>(null);const [next,setNext]=useState<(()=>void)|null>(null);
 const [departmentName,setDepartmentName]=useState('');const [departmentId,setDepartmentId]=useState('');const [member,setMember]=useState<Member|null>(null);const [identityIds,setIdentityIds]=useState<string[]>([]);
 const [invitation,setInvitation]=useState('');const [error,setError]=useState('');const [status,setStatus]=useState('');const [pending,setPending]=useState(false);const [loading,setLoading]=useState(true);
 const [memberSearch,setMemberSearch]=useState('');const [memberPage,setMemberPage]=useState(1);
 const [eventSearch,setEventSearch]=useState('');const [eventAction,setEventAction]=useState('');
 const [groupOperation,setGroupOperation]=useState('add');const [groupTarget,setGroupTarget]=useState('');const [groupSource,setGroupSource]=useState('');
 const dirty=!same(selected,draft);
 useLayoutEffect(()=>{onDirty(dirty);},[dirty,onDirty]);
 function fail(e:unknown){if(e instanceof WorkspaceError&&e.status===401){onUnauthorized();return;}setError(e instanceof Error?e.message:'服务暂时不可用，请稍后重试');}
 async function refresh(){
  setLoading(true);
  try{
   const [ds,ids,ts,ps,ms,es]=await Promise.all([
    workspaceApi<{items:Department[]}>('personnel/departments'),workspaceApi<PageData<Definition>>('personnel/identities?pageSize=100'),
    workspaceApi<PageData<Definition>>('personnel/templates?pageSize=100'),workspaceApi<{items:Permission[]}>('personnel/permissions'),
    workspaceApi<PageData<Member>>('personnel/members'),workspaceApi<PageData<Activity>>('personnel/events')]);
   setDepartments(ds.items);setIdentities(ids);setTemplates(ts);setPermissions(ps.items);setMembers(ms);setEvents(es);
   if(!departmentId)setDepartmentId(ds.items.find(v=>v.isRoot)?.id||'');
  }catch(e){fail(e);}finally{setLoading(false);}
 }
 useEffect(()=>{void refresh();},[]);
 useEffect(()=>{let active=true;const query=new URLSearchParams({search:memberSearch,page:String(memberPage)});workspaceApi<PageData<Member>>('personnel/members?'+query).then(value=>{if(active)setMembers(value);}).catch(e=>{if(active)fail(e);});return()=>{active=false;};},[memberSearch,memberPage]);
 useEffect(()=>{let active=true;const query=new URLSearchParams({search:eventSearch,action:eventAction});workspaceApi<PageData<Activity>>('personnel/events?'+query).then(value=>{if(active)setEvents(value);}).catch(e=>{if(active)fail(e);});return()=>{active=false;};},[eventSearch,eventAction]);
 function guarded(action:()=>void){if(dirty){setNext(()=>action);setDialog('dirty');}else action();}
 function changeTab(value:Tab){guarded(()=>{setTab(value);setSelected(null);setDraft(null);setError('');setStatus('');});}
 function choose(value:Definition){guarded(()=>{setSelected(structuredClone(value));setDraft(structuredClone(value));setError('');setStatus('');});}
 async function save(){
  if(!draft||pending)return;if(!draft.name.trim()){setError('请填写名称');setDialog(null);return;}setPending(true);setError('');
  try{
   const body={name:draft.name,description:draft.description,...(selected?{version:draft.version}:{}),...(tab==='身份'?{templateIds:draft.templateIds||[]}:{}),permissionCodes:draft.permissionCodes};
   const value=await workspaceApi<Definition>('personnel/'+(tab==='身份'?'identities':'templates')+(selected?'/'+draft.id:''),selected?'PUT':'POST',body);
   setSelected(value);setDraft(value);setDialog(null);await refresh();setStatus('已保存');
  }catch(e){setDialog(null);fail(e);}finally{setPending(false);}
 }
 async function createDepartment(){
  if(pending)return;if(!departmentName.trim()){setError('请填写部门名称');return;}setPending(true);setError('');
  try{await workspaceApi('personnel/departments','POST',{name:departmentName,parentId:departmentId});setDialog(null);setDepartmentName('');setStatus('部门已创建');await refresh();}catch(e){fail(e);}finally{setPending(false);}
 }
 async function createInvitation(){
  if(pending||invitation)return;setPending(true);setError('');
  try{const value=await workspaceApi<{id:string;code:string}>('invitations','POST',{});setInvitation(value.code);}catch(e){fail(e);}finally{setPending(false);}
 }
 async function assignIdentities(){
  if(!member||pending)return;setPending(true);setError('');
  try{await workspaceApi('personnel/members/'+member.id+'/identities','PUT',{identityIds,version:member.version});setDialog(null);setStatus('身份已分配');await refresh();}catch(e){fail(e);}finally{setPending(false);}
 }
 function newIdentity(){guarded(()=>{setSelected(null);setDraft({id:'',name:'',description:'',version:0,templateIds:[],permissionCodes:[],affectedMembers:0,affectedIdentities:0});setError('');setStatus('');});}
 async function adjustGroups(){
  if(!member||pending)return;if(!groupTarget||(groupOperation==='move'&&!groupSource)){setError('请选择部门');return;}setPending(true);setError('');
  try{await workspaceApi('personnel/members/'+member.id+'/groups','POST',{operation:groupOperation,departmentId:groupTarget,...(groupOperation==='move'?{sourceDepartmentId:groupSource}:{}),version:member.version});setDialog(null);await refresh();setStatus('分组已调整');}catch(e){fail(e);}finally{setPending(false);}
 }
 async function deleteTemplate(){
  if(!selected||pending)return;setPending(true);setError('');
  try{await workspaceApi('personnel/templates/'+selected.id+'?version='+selected.version,'DELETE');setSelected(null);setDraft(null);setDialog(null);await refresh();setStatus('模板已删除');}catch(e){setDialog(null);fail(e);}finally{setPending(false);}
 }
 const definition=tab==='身份'||tab==='权限模板';
 const usingTemplates=templates.items.filter(t=>draft?.templateIds?.includes(t.id));
 const direct=new Set(draft?.permissionCodes||[]);const effective=permissions.filter(p=>direct.has(p.code)||usingTemplates.some(t=>t.permissionCodes.includes(p.code)));
 return <div className={'admin-shell'+(side?'':' side-collapsed')+(top?'':' top-collapsed')}>
  <header className="admin-header"><div className="admin-corner"><img className="admin-settings-icon" src={settingsIcon} width="20" height="20" alt=""/>{top&&<strong>管理后台</strong>}</div>
   {top&&<div className="admin-actions"><button className="admin-action save" disabled={!dirty||pending} onClick={()=>setDialog('impact')}><img src={saveIcon} width="16" height="16" alt=""/>保存</button><button className="admin-action" onClick={onExit} disabled={pending}><img src={exitIcon} width="16" height="16" alt=""/>退出</button></div>}
   <button className="top-trigger" aria-label={top?'收起顶栏':'展开顶栏'} onClick={()=>setTop(!top)}><img src={grip} width="16" height="16" alt=""/></button>
  </header>
  <aside className="admin-sidebar">{side&&<span className="sidebar-caption">管理功能</span>}<button className="personnel-nav" aria-label="人员管理" onClick={()=>guarded(()=>{setTab('成员与部门');setDraft(null);setSelected(null);})}><img className="personnel-nav-icon" src={usersIcon} width="20" height="20" alt=""/>{side&&<span>人员管理</span>}</button>{side&&<p className="sidebar-caption">成员 · 部门 · 身份 · 权限</p>}
   <button className="side-trigger" aria-label={side?'收起侧栏':'展开侧栏'} onClick={()=>setSide(!side)}><img src={chevron} width="16" height="16" alt=""/></button>
  </aside>
  <main className="personnel-content">
   <div className="personnel-page-heading"><h1>人员管理</h1><p>统一管理成员、部门、身份与应用访问。</p></div>
   <nav className="personnel-tabs" role="tablist" aria-label="人员管理视图">{(['成员与部门','身份','权限模板','操作记录'] as Tab[]).map(value=><button key={value} role="tab" aria-selected={tab===value} aria-controls="personnel-panel" onClick={()=>changeTab(value)}>{value}</button>)}</nav>
   {error&&<p className="workspace-error" role="alert">{error}<button className="text-button" onClick={()=>{setError('');void refresh();}}>重新加载</button></p>}{status&&<p className="workspace-status" role="status">{status}</p>}
   <section id="personnel-panel" role="tabpanel" aria-label={tab} className="personnel-panel">
    <div className="section-heading"><div><h2>{tab}</h2><p>{tab==='成员与部门'?'管理成员归属与身份，部门调整不改变访问权限':tab==='身份'?'先配置身份，再把身份分配给成员':tab==='权限模板'?'集中配置中央权限，供多个身份复用':'查看成员、部门与权限配置的变更记录'}</p></div>{tab==='身份'&&<button className="admin-button primary" onClick={newIdentity}>新建身份</button>}{tab==='成员与部门'&&<><button className="admin-button" onClick={()=>{setDepartmentName('');setDialog('department');}}><img src={plusIcon} width="16" height="16" alt=""/>新建部门</button><button className="admin-button primary" onClick={()=>{setInvitation('');setDialog('invitation');}}><img src={plusWhite} width="16" height="16" alt=""/>邀请成员</button></>}</div>
    {loading?<p role="status">正在加载…</p>:tab==='成员与部门'?<div className="personnel-workspace">
     <section className="department-panel surface"><h3>部门 <span className="badge">{departments.length}</span></h3><button className="department-row selected">全部成员 <small>{members.total}</small></button>{departments.map(d=><button key={d.id} className="department-row" style={{paddingLeft:d.parentId?28:12}} onClick={()=>setDepartmentId(d.id)}>{d.name}<small>{d.memberCount}</small></button>)}<p className="panel-note">部门只用于分组。<br/>移动成员不会自动改变<br/>身份或应用访问权限。</p></section>
     <section className="member-table surface"><div className="table-toolbar"><div className="admin-search"><img src={searchIcon} width="16" height="16" alt=""/><input aria-label="搜索成员" placeholder="搜索成员" value={memberSearch} onChange={e=>{setMemberSearch(e.target.value);setMemberPage(1);}}/></div><span>{members.total} 位成员</span></div><div className="table-scroll"><table><thead><tr><th className="member-selector"></th><th>成员</th><th>部门</th><th>身份</th><th>人员管理</th><th>操作</th></tr></thead><tbody>{members.items.map(m=><tr key={m.id}><td></td><td><div className="member-name"><span className="member-avatar">{Array.from(m.account)[0]}</span><div><strong>{m.account}</strong><small>{m.status==='active'?'正常':'已停用'}</small></div></div></td><td>{m.departments.map(d=>d.name).join('、')||'未分组'}</td><td>{m.bootstrapAdmin?'Bootstrap Admin':m.identities.map(i=>i.name).join('、')||'未分配'}</td><td>{m.bootstrapAdmin?'默认拥有':m.permissions.some(p=>p.code==='personnel.manage')?'已开启':'未开启'}</td><td><button className="text-button" onClick={()=>{setMember(m);setIdentityIds([...m.identityIds]);setDialog('member');}}>配置身份</button><button className="text-button" onClick={()=>{setMember(m);setGroupOperation('add');setGroupTarget('');setGroupSource(m.departmentIds[0]||'');setDialog('groups');}}>调整分组</button></td></tr>)}</tbody></table>{!members.total&&<p className="empty-state">暂无成员</p>}</div><div className="table-footer"><span>共 {members.total} 位成员</span><span>第 {memberPage} 页 / 共 {Math.max(1,Math.ceil(members.total/members.pageSize))} 页</span><button className="text-button" disabled={memberPage<=1} onClick={()=>setMemberPage(v=>v-1)}>上一页</button><button className="text-button" disabled={memberPage*members.pageSize>=members.total} onClick={()=>setMemberPage(v=>v+1)}>下一页</button></div></section>
    </div>:definition?<div className="personnel-workspace">
     <section className="definition-list surface"><div className="admin-search"><img src={searchIcon} width="16" height="16" alt=""/><input aria-label={tab==='身份'?'搜索身份':'搜索权限模板'} placeholder={tab==='身份'?'搜索身份':'搜索权限模板'}/></div>{(tab==='身份'?identities:templates).items.map(item=><button key={item.id} aria-label={item.name} className={'definition-item'+(draft?.id===item.id?' selected':'')} onClick={()=>choose(item)}><strong>{item.name}</strong><span>{item.description||'暂无说明'}</span><small>{tab==='身份'?item.affectedMembers+' 位成员':item.affectedIdentities+' 个身份引用'} · {item.permissionCodes.length} 项直接权限</small></button>)}{!(tab==='身份'?identities:templates).total&&<p className="empty-state">{tab==='身份'?'暂无身份':'暂无权限模板'}</p>}<p className="panel-note">{tab==='身份'?<>身份是可分配给成员的模板。<br/>同一个身份可供多位成员使用。</>:<>一个权限模板可供多个身份复用。<br/>共享修改会影响所有引用者。</>}</p></section>
     {draft?<section className="definition-details surface"><div className="detail-heading"><h2>{selected?.name||(tab==='身份'?'新建身份':'新建权限模板')}</h2><span className="badge">{tab==='身份'?draft.affectedMembers+' 位成员使用':draft.affectedIdentities+' 个身份正在引用'}</span></div><div className="basic-information"><label>{tab==='身份'?'身份名称':'模板名称'}<input aria-label={tab==='身份'?'身份名称':'模板名称'} value={draft.name} maxLength={100} onChange={e=>setDraft({...draft,name:e.target.value})}/></label><label>说明<input aria-label="说明" value={draft.description} maxLength={1000} onChange={e=>setDraft({...draft,description:e.target.value})}/></label></div>
      {tab==='身份'&&<section className="config-section"><h3>权限模板</h3>{templates.items.map(t=><label key={t.id} className="template-choice"><input type="checkbox" aria-label={'模板：'+t.name} checked={draft.templateIds?.includes(t.id)||false} onChange={()=>setDraft({...draft,templateIds:toggle(draft.templateIds||[],t.id)})}/><strong>{t.name}</strong><small>{t.permissionCodes.length} 项权限</small></label>)}{!templates.total&&<p>暂无权限模板</p>}</section>}
      <section className="config-section"><h3>{tab==='身份'?'直接权限':'权限配置'}</h3><p>{tab==='身份'?'可直接选择单项权限，与权限模板合并生效。':'系统管理'}</p>{permissions.filter(p=>p.category==='system').map(p=><label key={p.code} className="permission-item"><input type="checkbox" aria-label={(tab==='身份'?'直接权限：':'中央权限：')+p.name} checked={direct.has(p.code)} onChange={()=>setDraft({...draft,permissionCodes:toggle(draft.permissionCodes,p.code)})}/><span><strong>{p.name}</strong><span>邀请成员、管理部门、身份与权限模板</span>{usingTemplates.filter(t=>t.permissionCodes.includes(p.code)).map(t=><small key={t.id}>来自模板：{t.name}</small>)}</span></label>)}</section>
      {tab==='身份'&&<div className="effective-permissions"><strong>有效权限</strong>{effective.map(p=><span key={p.code} className="badge">{p.name}</span>)}</div>}
      <section className="config-section"><h3>应用访问</h3>{permissions.some(p=>p.category==='application')?permissions.filter(p=>p.category==='application').map(p=><label key={p.code} className="permission-item"><input aria-label={(tab==='身份'?'直接权限：':'中央权限：')+p.name} type="checkbox" checked={direct.has(p.code)} onChange={()=>setDraft({...draft,permissionCodes:toggle(draft.permissionCodes,p.code)})}/>{p.name}</label>):<div className="no-applications"><strong>尚未接入业务应用</strong><p>接入知识库或自建应用后，可在此配置使用权限。</p><small>文档、分类和表单的内部权限，由各应用自己管理。</small></div>}</section>
      <p className="panel-note">配置完成后点击右上角「保存」，将同步影响所有引用者。</p>{tab==='权限模板'&&<button className="text-button danger" disabled={!selected||dirty||draft.affectedIdentities>0} onClick={()=>setDialog('delete')}>删除模板</button>}
     </section>:<section className="definition-details surface empty-state">请选择{tab==='身份'?'身份':'权限模板'}查看配置</section>}
    </div>:<section className="activity-table surface"><div className="table-toolbar"><div className="admin-search"><img src={searchIcon} width="16" height="16" alt=""/><input aria-label="搜索操作记录" placeholder="搜索成员或操作对象" value={eventSearch} onChange={e=>setEventSearch(e.target.value)}/></div><label>操作类型<select aria-label="操作类型" value={eventAction} onChange={e=>setEventAction(e.target.value)}><option value="">全部操作</option>{['DEPARTMENT_CREATED','DEPARTMENT_UPDATED','DEPARTMENT_DELETED','IDENTITY_CREATED','IDENTITY_UPDATED','IDENTITY_DELETED','TEMPLATE_CREATED','TEMPLATE_UPDATED','TEMPLATE_DELETED','MEMBER_IDENTITIES_UPDATED','MEMBER_GROUPS_UPDATED','INVITATION_CREATED'].map(action=><option key={action} value={action}>{action}</option>)}</select></label><span>最近 7 天</span></div><div className="table-scroll"><table><thead><tr>{['时间','操作者','操作','对象','变更内容','结果'].map(h=><th key={h}>{h}</th>)}</tr></thead><tbody>{events.items.map(event=><tr key={event.id}><td>{new Date(event.occurredAt).toLocaleString('zh-CN')}</td><td>{event.actorAccount}</td><td>{event.action}</td><td>{event.objectType}</td><td>{JSON.stringify(event.summary)}</td><td>{event.outcome==='success'?'已完成':'失败'}</td></tr>)}</tbody></table>{!events.total&&<p className="empty-state">暂无操作记录</p>}</div><p className="activity-note">记录仅包含配置变更，不展示密码或邀请码明文。</p></section>}
   </section>
  </main>
  {dialog&&<Modal title={dialog==='dirty'?'有未保存的修改':dialog==='impact'?'确认共享修改':dialog==='department'?'新建部门':dialog==='invitation'?'邀请成员':dialog==='groups'?'调整成员分组':dialog==='delete'?'删除权限模板':'配置成员身份'} busy={pending} onClose={()=>{setDialog(null);setNext(null);setInvitation('');}}>
   {dialog==='groups'?<><p>{member?.account}：部门调整不会改变身份或应用访问。</p><label>分组操作<select aria-label="分组操作" value={groupOperation} onChange={e=>setGroupOperation(e.target.value)}><option value="add">添加</option><option value="remove">移除</option><option value="move">移动</option></select></label>{groupOperation==='move'&&<label>来源部门<select aria-label="来源部门" value={groupSource} onChange={e=>setGroupSource(e.target.value)}><option value="">请选择</option>{departments.filter(d=>member?.departmentIds.includes(d.id)).map(d=><option key={d.id} value={d.id}>{d.name}</option>)}</select></label>}<label>目标部门<select aria-label="目标部门" value={groupTarget} onChange={e=>setGroupTarget(e.target.value)}><option value="">请选择</option>{departments.map(d=><option key={d.id} value={d.id}>{d.name}</option>)}</select></label><div className="dialog-actions"><button className="admin-button primary" disabled={pending} onClick={()=>void adjustGroups()}>确认调整</button></div></>:dialog==='delete'?<><p>确认删除“{selected?.name}”？存在引用时无法删除。</p><div className="dialog-actions"><button className="admin-button" onClick={()=>setDialog(null)}>取消</button><button className="admin-button primary" disabled={pending} onClick={()=>void deleteTemplate()}>确认删除</button></div></>:dialog==='dirty'?<><p>切换将丢失未保存的修改。</p><div className="dialog-actions"><button className="admin-button" onClick={()=>{setDialog(null);setNext(null);}}>继续编辑</button><button className="admin-button primary" onClick={()=>{setDialog(null);setDraft(selected);next?.();setNext(null);}}>放弃修改</button></div></>:dialog==='impact'?<><p>本次修改将同步影响 {draft?.affectedMembers||0} 位成员{tab==='权限模板'?'，以及 '+(draft?.affectedIdentities||0)+' 个身份':''}。</p><div className="dialog-actions"><button className="admin-button" onClick={()=>setDialog(null)}>继续编辑</button><button className="admin-button primary" disabled={pending} onClick={()=>void save()}>确认保存</button></div></>:dialog==='department'?<><label>部门名称<input aria-label="部门名称" value={departmentName} maxLength={100} onChange={e=>setDepartmentName(e.target.value)}/></label><label>父部门<select aria-label="父部门" value={departmentId} onChange={e=>setDepartmentId(e.target.value)}>{departments.map(d=><option key={d.id} value={d.id}>{d.name}</option>)}</select></label><div className="dialog-actions"><button className="admin-button primary" disabled={pending} onClick={()=>void createDepartment()}>确认创建</button></div></>:dialog==='invitation'?<><p>生成单次邀请码，注册成功后不可再次使用。请安全转交给受邀成员。</p>{invitation?<label>邀请码<input aria-label="邀请码" readOnly value={invitation}/></label>:<button className="admin-button primary" disabled={pending} onClick={()=>void createInvitation()}>生成邀请码</button>}</>:<><p>{member?.account}</p>{identities.items.map(i=><label key={i.id} className="template-choice"><input type="checkbox" aria-label={'身份：'+i.name} checked={identityIds.includes(i.id)} onChange={()=>setIdentityIds(toggle(identityIds,i.id))}/>{i.name}</label>)}{!identities.total&&<p>暂无可分配身份</p>}<div className="dialog-actions"><button className="admin-button primary" disabled={pending} onClick={()=>void assignIdentities()}>确认分配</button></div></>}
   {error&&<p className="workspace-error" role="alert">{error}</p>}
  </Modal>}
 </div>;
}
