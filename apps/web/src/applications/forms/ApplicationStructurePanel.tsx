import {useEffect,useRef,useState} from 'react';
import {formApi,formErrorText,FormApiError,type FormWrite,type FormUpdate,type StructureWrite} from './api';
import type {Directory,FormSource,Structure,UUID} from './contracts';
import {FormsDialog} from './FormsDialog';
import './forms.css';

export type ApplicationStructureProps={appId:string;onOpenForm?:(viewId:string)=>void;onDirtyChange?:(dirty:boolean)=>void};
type Selected={kind:'directory'|'form';id:UUID}|null;
type Dialog='directory'|'renameDirectory'|'moveDirectory'|'form'|'editForm'|null;
type Pending={operationId:UUID;run:()=>Promise<unknown>};
const uuid=()=>crypto.randomUUID();
const descendants=(items:Directory[],source:UUID,target:UUID|null)=>{const seen=new Set<UUID>();
  while(target){if(target===source||seen.has(target))return true;seen.add(target);target=items.find(item=>item.id===target)?.parentId??null;}return false;};
const position=(items:{position:number}[])=>Math.max(-1,...items.map(item=>item.position))+1;
export function ApplicationStructurePanel({appId,onOpenForm,onDirtyChange}:ApplicationStructureProps){
  const [structure,setStructure]=useState<Structure|null>(null),[selected,setSelected]=useState<Selected>(null);
  const [dialog,setDialog]=useState<Dialog>(null),[name,setName]=useState(''),[parentId,setParentId]=useState<UUID|null>(null);
  const [source,setSource]=useState<FormSource>({kind:'new_table'}),[error,setError]=useState(''),[notice,setNotice]=useState('');
  const [busy,setBusy]=useState(false),[unconfirmed,setUnconfirmed]=useState(false),[pending,setPending]=useState<Pending|null>(null);
  const [reload,setReload]=useState(0),request=useRef(0);
  useEffect(()=>{const current=++request.current,controller=new AbortController();setStructure(null);setError('');
    void formApi.structure(appId,controller.signal).then(value=>{if(current===request.current)setStructure(value);})
      .catch(problem=>{if(!controller.signal.aborted&&current===request.current)setError(formErrorText(problem));});
    return()=>{controller.abort();request.current++;};},[appId,reload]);
  useEffect(()=>onDirtyChange?.(!!dialog&&(!!name.trim()||unconfirmed)),[dialog,name,unconfirmed,onDirtyChange]);
  const folder=structure?.directories.find(item=>selected?.kind==='directory'&&item.id===selected.id);
  const form=structure?.forms.find(item=>selected?.kind==='form'&&item.id===selected.id);
  const canEdit=!!structure?.capabilities.canManageDefinition;
  const dirs=[...(structure?.directories??[])].sort((a,b)=>a.position-b.position);
  const open=(kind:Exclude<Dialog,null>,item?:{id:UUID;name:string;parentId?:UUID|null})=>{
    if(!canEdit)return;setDialog(kind);setError('');setNotice('');setUnconfirmed(false);setPending(null);
    setName(kind==='renameDirectory'||kind==='moveDirectory'||kind==='editForm'?item?.name??'':'');
    setParentId(kind==='directory'?item?.id??null:item?.parentId??null);setSource({kind:'new_table'});
  };
  const refresh=()=>formApi.structure(appId).then(setStructure);
  const apply=async(work:Pending)=>{setPending(work);setBusy(true);setError('');
    try{await work.run();setPending(null);setUnconfirmed(false);setDialog(null);setName('');setNotice('已保存');
      try{await refresh();}catch(problem){setError(`更改已确认，目录刷新失败：${formErrorText(problem)}`);}}
    catch(problem){if(problem instanceof FormApiError&&(problem.status===0||problem.code==='APPLICATION_OPERATION_UNCONFIRMED')){
      setUnconfirmed(true);setError('网络连接中断，操作结果暂未确认；请查询原操作或按原请求重试');
    }else setError(formErrorText(problem));}
    finally{setBusy(false);}
  };
  const submit=async()=>{if(!structure||!dialog||busy||unconfirmed)return;const clean=name.trim();
    if(!clean&&dialog!=='moveDirectory'){setError('请输入名称');return;}
    const operationId=uuid(),expectedStructureVersion=structure.structureVersion;
    let work:Pending;
    if(dialog==='directory'){
      const input:StructureWrite={operationId,name:clean,parentId,position:position(structure.directories.filter(item=>item.parentId===parentId)),expectedStructureVersion};
      work={operationId,run:()=>formApi.createDirectory(appId,input)};
    }else if(dialog==='renameDirectory'||dialog==='moveDirectory'){
      if(!folder)return;
      if(descendants(structure.directories,folder.id,parentId)){setError('目录不能移动到自己或自己的子目录');return;}
      const input:StructureWrite={operationId,name:dialog==='renameDirectory'?clean:folder.name,parentId,
        position:parentId===folder.parentId?folder.position:position(structure.directories.filter(item=>item.parentId===parentId)),expectedStructureVersion};
      work={operationId,run:()=>formApi.updateDirectory(appId,folder.id,input)};
    }else if(dialog==='form'){
      if(source.kind==='existing_table'&&!structure.tables.some(item=>item.id===source.tableId)){setError('请选择此应用中的逻辑表');return;}
      const input:FormWrite={operationId,name:clean,source,directoryId:parentId,
        position:position(structure.forms.filter(item=>item.directoryId===parentId)),expectedStructureVersion};
      work={operationId,run:()=>formApi.createForm(appId,input)};
    }else{
      if(!form)return;
      const input:FormUpdate={operationId,name:clean,directoryId:parentId,
        position:parentId===form.directoryId?form.position:position(structure.forms.filter(item=>item.directoryId===parentId)),expectedStructureVersion};
      work={operationId,run:()=>formApi.updateForm(appId,form.id,input)};
    }
    await apply(work);
  };
  const check=async()=>{if(!pending)return;setBusy(true);setError('');
    try{const value=await formApi.operation(pending.operationId);if(value.status==='confirmed'){
      setPending(null);setUnconfirmed(false);setDialog(null);setName('');setNotice('原操作已确认');
      try{await refresh();}catch(problem){setError(`原操作已确认，目录刷新失败：${formErrorText(problem)}`);}return;}
      setError('原操作状态暂不能确认，请保留当前输入');
    }catch(problem){setError(problem instanceof FormApiError&&problem.status===404?
      '暂未查到原操作；这不能证明操作已回滚，可按原请求重试':formErrorText(problem));}
    finally{setBusy(false);}
  };
  const tree=(parent:UUID|null,seen=new Set<UUID>()):React.ReactNode=>{
    if(!structure)return null;
    return <ul className="forms-tree-list" role={parent===null?'tree':'group'} aria-label={parent===null?'应用目录':undefined}>
      {dirs.filter(item=>item.parentId===parent&&!seen.has(item.id)).map(item=><li role="treeitem" aria-label={item.name} aria-expanded="true" className="forms-tree-item" key={item.id}>
        <div className={`forms-tree-row${selected?.id===item.id?' active':''}`}>
          <button type="button" onClick={()=>setSelected({kind:'directory',id:item.id})}>▾ 📁 {item.name}</button>
          {canEdit&&<button type="button" aria-label={`移动目录 ${item.name}`} onClick={()=>{setSelected({kind:'directory',id:item.id});open('moveDirectory',item);}}>↔</button>}
        </div>{tree(item.id,new Set([...seen,item.id]))}</li>)}
      {structure.forms.filter(item=>item.directoryId===parent).sort((a,b)=>a.position-b.position).map(item=><li role="treeitem" aria-label={item.name} className="forms-tree-item" key={item.id}>
        <div className={`forms-tree-row${selected?.id===item.id?' active':''}`}>
          <button type="button" onClick={()=>setSelected({kind:'form',id:item.id})}>▤ {item.name}</button>
          <button type="button" aria-label={`打开表单 ${item.name}`} onClick={()=>onOpenForm?.(item.id)}>打开</button>
        </div></li>)}
      {structure.tables.filter(item=>item.directoryId===parent&&!structure.forms.some(view=>view.tableId===item.id)).map(item=>
        <li role="treeitem" aria-label={item.name} key={item.id}><div className="forms-tree-row">▦ {item.name}</div></li>)}
    </ul>;
  };
  if(!structure)return <section className="forms-module forms-loading" role="status">{error?<><p role="alert">{error}</p><button type="button" onClick={()=>setReload(value=>value+1)}>重试</button></>:'正在加载应用目录…'}</section>;
  return <section className="forms-module" aria-label="目录与视图管理">
    <div className="forms-structure-head"><div><strong>应用目录与视图</strong><p className="forms-muted">管理目录、逻辑表和表单视图</p></div>
      <div className="forms-structure-actions"><button type="button" disabled={!canEdit} onClick={()=>open('directory',folder)}>新建目录</button>
      <button type="button" className="forms-primary" disabled={!canEdit} onClick={()=>open('form',form??folder)}>新建表单</button></div></div>
    {error&&!dialog&&<p className="forms-alert" role="alert">{error}</p>}{notice&&<p className="forms-success" role="status">{notice}</p>}
    <div className="forms-structure-layout"><nav className="forms-structure-tree">{tree(null)}</nav>
      <section className="forms-structure-detail" aria-label="目录与视图详情">
      {folder?<><h2>📁 {folder.name}</h2><p>位置 {folder.position+1}</p>{canEdit&&<>
        <button type="button" onClick={()=>open('renameDirectory',folder)}>重命名目录</button>
        <button type="button" onClick={()=>open('moveDirectory',folder)}>移动目录</button>
        <button type="button" onClick={()=>open('directory',folder)}>创建子目录</button></>}</>:
      form?<><h2>▤ {form.name}</h2><p>逻辑表：{structure.tables.find(item=>item.id===form.tableId)?.name??form.tableId}</p>
        <p>视图版本：{form.viewVersion}</p><button type="button" className="forms-primary" onClick={()=>onOpenForm?.(form.id)}>打开表单设计器</button>
        {canEdit&&<button type="button" onClick={()=>open('editForm',{...form,parentId:form.directoryId})}>编辑表单名称与位置</button>}</>:
      <><h2>选择目录或视图</h2><p className="forms-muted">从左侧选择项目，查看详情并管理位置。</p></>}</section></div>
    {dialog&&<FormsDialog title={dialog==='directory'?'新建目录':dialog==='renameDirectory'?'重命名目录':dialog==='moveDirectory'?'移动目录':dialog==='form'?'新建表单':'编辑表单'}
      onClose={()=>{if(!busy&&!unconfirmed)setDialog(null);}} busy={busy||unconfirmed}>
      {dialog!=='moveDirectory'&&<label>{dialog==='directory'||dialog==='renameDirectory'?'目录名称':'表单名称'}
        <input aria-label={dialog==='directory'||dialog==='renameDirectory'?'目录名称':'表单名称'} value={name} disabled={busy||unconfirmed} onChange={event=>setName(event.target.value)}/></label>}
      {dialog!=='renameDirectory'&&<label>所属目录<select aria-label="所属目录" value={parentId??''} disabled={busy||unconfirmed} onChange={event=>setParentId(event.target.value||null)}>
        <option value="">根目录</option>{dirs.filter(item=>dialog!=='moveDirectory'||!folder||!descendants(dirs,folder.id,item.id))
          .map(item=><option key={item.id} value={item.id}>{item.name}</option>)}</select></label>}
      {dialog==='form'&&<fieldset><legend>表单来源</legend>
        <label className="forms-checkbox"><input type="radio" checked={source.kind==='new_table'} disabled={busy||unconfirmed} onChange={()=>setSource({kind:'new_table'})}/>新建逻辑表及首个表单视图</label>
        <label className="forms-checkbox"><input type="radio" checked={source.kind==='existing_table'} disabled={busy||unconfirmed||!structure.tables.length}
          onChange={()=>setSource({kind:'existing_table',tableId:structure.tables[0].id})}/>使用现有逻辑表</label>
        {source.kind==='existing_table'&&<select aria-label="现有逻辑表" value={source.tableId} disabled={busy||unconfirmed}
          onChange={event=>setSource({kind:'existing_table',tableId:event.target.value})}>
          {structure.tables.map(item=><option key={item.id} value={item.id}>{item.name}</option>)}</select>}</fieldset>}
      {error&&<p className="forms-alert" role="alert">{error}</p>}
      {unconfirmed&&pending?<div className="forms-dialog-actions">
        <button type="button" disabled={busy} onClick={()=>void check()}>查询原操作结果</button>
        <button type="button" disabled={busy} onClick={()=>void apply(pending)}>按原请求重试</button></div>:
      <div className="forms-dialog-actions"><button type="button" disabled={busy} onClick={()=>setDialog(null)}>取消</button>
        <button type="button" className="forms-primary" disabled={busy} onClick={()=>void submit()}>{dialog==='directory'?'创建目录':dialog==='form'?'创建表单':'保存变更'}</button></div>}
      </FormsDialog>}
  </section>;
}
