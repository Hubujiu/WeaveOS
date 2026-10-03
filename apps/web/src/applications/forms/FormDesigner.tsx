import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type DragEvent, type SetStateAction } from 'react';
import { formApi, formErrorText, FormApiError, immutablePacket, validSaveResult,
  type ReferenceCandidate, type SaveWrite } from './api';
import type { Definition, DefinitionInput, FieldInput, FieldKind, LayoutNodeInput, Preflight, UUID } from './contracts';
import { FieldRenderer, FormPreview } from './FieldRenderer';
import { FormsDialog } from './FormsDialog';
import {createLeaveController,type LeaveController,type LeaveGuardProps,type LeaveStatus} from './leaveGuard';
import './forms.css';

export type FormDesignerProps = LeaveGuardProps&{
  appId:string;actorId:string;viewId:string;onDirtyChange?:(dirty:boolean)=>void;onBack?:()=>void;
  onUnauthorized?:()=>void;onIdentityMismatch?:()=>void;
};
type PaletteKind=FieldKind|'group'|'divider'|'description'|'system_field';
const palette:{kind:PaletteKind;label:string}[]=[
  {kind:'text',label:'文本'},{kind:'multiline',label:'多行文本'},
  {kind:'number',label:'数字'},{kind:'money',label:'金额'},
  {kind:'date',label:'日期'},{kind:'datetime',label:'日期时间'},
  {kind:'single_select',label:'单选'},{kind:'multi_select',label:'多选'},
  {kind:'boolean',label:'布尔'},{kind:'member',label:'成员'},
  {kind:'department',label:'部门'},{kind:'group',label:'分组'},
  {kind:'divider',label:'分割线'},{kind:'description',label:'说明'},
  {kind:'system_field',label:'系统字段'},
];
const labelFor=Object.fromEntries(palette.map(item=>[item.kind,item.label]));
const uuid=()=>crypto.randomUUID();
const dataKind=(kind:PaletteKind):kind is FieldKind=>
  !['group','divider','description','system_field'].includes(kind);
function makeField(kind:FieldKind):FieldInput {
  const shared={id:uuid(),name:`新建${labelFor[kind]}`,required:false,default:null,
    presentation:{helpText:null,displayTimeZone:null}};
  switch(kind){
    case 'text':case 'multiline':return {...shared,kind,config:{maxLength:null}};
    case 'number':case 'money':return {...shared,kind,config:{}};
    case 'date':return {...shared,kind,config:{}};
    case 'datetime':return {...shared,kind,config:{}};
    case 'single_select':case 'multi_select':return {...shared,kind,config:{options:[]}};
    case 'boolean':return {...shared,kind,config:{}};
    case 'member':case 'department':return {...shared,kind,config:{}};
  }
}
function fromDefinition(base:Definition):DefinitionInput {
  return {expectedSchemaVersion:base.table.schemaVersion,expectedViewVersion:base.form.viewVersion,
    fields:structuredClone(base.fields),layout:structuredClone(base.layout),optionMappings:[]};
}
function definitionDirty(base:Definition|null,draft:DefinitionInput|null):boolean {
  return !!base&&!!draft&&(!base.table.schemaReady||
    JSON.stringify(draft.fields)!==JSON.stringify(base.fields)||
    JSON.stringify(draft.layout)!==JSON.stringify(base.layout)||draft.optionMappings.length>0);
}
function mapLayout(nodes:LayoutNodeInput[],fn:(node:LayoutNodeInput)=>LayoutNodeInput):LayoutNodeInput[]{
  return nodes.map(node=>fn(node.kind==='group'?{...node,children:mapLayout(node.children,fn)}:node));
}
function findNode(nodes:LayoutNodeInput[],id:UUID):LayoutNodeInput|undefined {
  for(const node of nodes){if(node.id===id)return node;
    if(node.kind==='group'){const nested=findNode(node.children,id);if(nested)return nested;}}
}
function removeNode(nodes:LayoutNodeInput[],id:UUID):LayoutNodeInput[]{
  return nodes.filter(node=>node.id!==id).map(node=>node.kind==='group'?{...node,children:removeNode(node.children,id)}:node);
}
function moveNode(nodes:LayoutNodeInput[],id:UUID,shift:number):LayoutNodeInput[]{
  const copy=[...nodes],index=copy.findIndex(node=>node.id===id);
  if(index!==-1){const next=index+shift;if(next>=0&&next<copy.length)[copy[index],copy[next]]=[copy[next],copy[index]];return copy;}
  return copy.map(node=>node.kind==='group'?{...node,children:moveNode(node.children,id,shift)}:node);
}
function insertAfter(nodes:LayoutNodeInput[],target:UUID,node:LayoutNodeInput):LayoutNodeInput[]{
  const index=nodes.findIndex(item=>item.id===target);
  if(index!==-1){const copy=[...nodes];copy.splice(index+1,0,node);return copy;}
  return nodes.map(item=>item.kind==='group'?{...item,children:insertAfter(item.children,target,node)}:item);
}
function parentGroup(nodes:LayoutNodeInput[],id:UUID,parent:UUID|null=null):UUID|null {
  for(const item of nodes){if(item.id===id)return parent;
    if(item.kind==='group'){const nested=parentGroup(item.children,id,item.id);if(nested!==null)return nested;}}
  return null;
}
function groupsIn(nodes:LayoutNodeInput[]):Extract<LayoutNodeInput,{kind:'group'}>[] {
  return nodes.flatMap(item=>item.kind==='group'?[item,...groupsIn(item.children)]:[]);
}
function systemCodesIn(nodes:LayoutNodeInput[]):string[] {
  return nodes.flatMap(item=>item.kind==='system_field'?[item.fieldId]:
    item.kind==='group'?systemCodesIn(item.children):[]);
}
function placedFields(nodes:LayoutNodeInput[]):Set<UUID> {
  const values=new Set<UUID>();
  const visit=(items:LayoutNodeInput[])=>items.forEach(item=>{
    if(item.kind==='field')values.add(item.fieldId);
    if(item.kind==='group')visit(item.children);
  });
  visit(nodes);return values;
}
function contains(nodes:LayoutNodeInput[],id:UUID):boolean {
  return nodes.some(item=>item.id===id||(item.kind==='group'&&contains(item.children,id)));
}
function unknownWrite(error:unknown):boolean {
  return error instanceof FormApiError&&(error.status===0||error.status===408||error.status>=500||
    error.code==='APPLICATION_OPERATION_UNCONFIRMED'||
    (error.status>=200&&error.status<300&&error.code==='INVALID_RESPONSE'));
}

type DesignerPhase='loading'|'idle'|'preflight'|'saving'|'unconfirmed';
type DesignerSnapshot={base:Definition|null;draft:DefinitionInput|null;phase:DesignerPhase;
  pending:SaveWrite|null;permissionRevoked:boolean;error:string;notice:string;
  selected:UUID|null;group:UUID|null};
const draftMemory=new Map<string,DesignerSnapshot>();
export function FormDesigner(props:FormDesignerProps){
  const scopeKey=JSON.stringify([props.actorId,props.appId,props.viewId]);
  return <FormDesignerScope key={scopeKey} {...props} scopeKey={scopeKey}/>;
}
function FormDesignerScope({appId,actorId,viewId,onDirtyChange,onBack,onUnauthorized,onIdentityMismatch,registerLeaveGuard,
  scopeKey}:FormDesignerProps&{scopeKey:string}){
  const restored=draftMemory.get(scopeKey);
  const [base,reactSetBase]=useState<Definition|null>(restored?.base??null),
    [draft,reactSetDraft]=useState<DefinitionInput|null>(restored?.draft??null);
  const [phase,reactSetPhase]=useState<DesignerPhase>(restored?.phase==='saving'?'unconfirmed':
    restored?.phase==='preflight'?'idle':restored?.phase??'loading');
  const [selected,setSelected]=useState<UUID|null>(restored?.selected??null),
    [group,setGroup]=useState<UUID|null>(restored?.group??null);
  const [dialog,setDialog]=useState<'preview'|'impact'|'dirty'|'conflict'|null>(null);
  const [leavePrompt,setLeavePrompt]=useState<LeaveStatus>('clean');
  const [impact,setImpact]=useState<Preflight|null>(null),[pending,reactSetPending]=useState<SaveWrite|null>(restored?.pending??null);
  const [impactStale,setImpactStale]=useState(false),[impactError,setImpactError]=useState('');
  const [error,setError]=useState(restored?.phase==='saving'?'保存结果暂未确认，请查询原操作':restored?.error??''),
    [notice,setNotice]=useState(restored?.notice??''),[reload,setReload]=useState(0);
  const [permissionRevoked,setPermissionRevoked]=useState(restored?.permissionRevoked??false);
  const [verified,setVerified]=useState(false),[verificationError,setVerificationError]=useState('');
  const requestKey=useRef(0);
  const saveEpoch=useRef(0),preflightAbort=useRef<AbortController|null>(null);
  const live=useRef({base,draft,phase,pending});
  const setBase=(next:Definition|null)=>{live.current.base=next;reactSetBase(next);};
  const setDraft=(update:SetStateAction<DefinitionInput|null>)=>{
    const next=typeof update==='function'?update(live.current.draft):update;
    live.current.draft=next;reactSetDraft(next);
  };
  const setPhase=(next:DesignerPhase)=>{live.current.phase=next;reactSetPhase(next);};
  const setPending=(next:SaveWrite|null)=>{live.current.pending=next;reactSetPending(next);};
  const leaveController=useRef<LeaveController|null>(null);
  const alive=useRef(true);
  const scope=useRef(scopeKey);
  const dirtyCallback=useRef(onDirtyChange);dirtyCallback.current=onDirtyChange;
  const reportAuth=(problem:unknown)=>{
    if(!(problem instanceof FormApiError))return;
    if(problem.status===401||problem.status===403||problem.code==='AUTH_SESSION_CHANGED'){
      setVerified(false);setVerificationError(formErrorText(problem));
    }
    if(problem.status===401)onUnauthorized?.();
    else if(problem.code==='AUTH_SESSION_CHANGED')onIdentityMismatch?.();
  };
  useEffect(()=>{alive.current=true;return()=>{alive.current=false;requestKey.current++;
    saveEpoch.current++;preflightAbort.current?.abort();preflightAbort.current=null;};},[]);
  useEffect(()=>()=>{dirtyCallback.current?.(false);},[]);
  useLayoutEffect(()=>{draftMemory.set(scopeKey,{base,draft,phase,pending,permissionRevoked,error,notice,selected,group});},
    [scopeKey,base,draft,phase,pending,permissionRevoked,error,notice,selected,group]);
  useEffect(()=>{
    const key=++requestKey.current,controller=new AbortController();
    const preserve=!!base&&!!draft&&(definitionDirty(base,draft)||!!pending);
    setVerified(false);setVerificationError('');
    void formApi.definition(appId,actorId,viewId,controller.signal).then(value=>{
      if(!alive.current||key!==requestKey.current)return;
      if(preserve&&!value.capabilities.canManageDefinition){
        setPermissionRevoked(true);setVerificationError('没有表单配置权限，已隐藏原草稿与操作');return;
      }
      setPermissionRevoked(false);
      if(preserve){
        setError(pending?'保存结果暂未确认，请查询原操作或按原请求重试':'');
        if(value.table.schemaVersion!==base.table.schemaVersion||value.form.viewVersion!==base.form.viewVersion)
          setError('服务器配置版本已变化，原草稿仍保留；请核查原操作或手动决定是否放弃草稿');
      }else{
        setBase(value);setDraft(fromDefinition(value));setPhase('idle');setSelected(null);setGroup(null);
        setPending(null);setImpact(null);setImpactStale(false);setImpactError('');setError('');setNotice('');setDialog(null);
      }
      setVerified(true);
    }).catch(problem=>{if(!alive.current||controller.signal.aborted||key!==requestKey.current)return;
      reportAuth(problem);
      setVerificationError(formErrorText(problem));});
    return()=>{controller.abort();requestKey.current++;};
  },[appId,actorId,viewId,reload,scopeKey]);
  const dirty=useMemo(()=>definitionDirty(base,draft),[base,draft]);
  const uncertain=phase==='unconfirmed'||phase==='saving'&&!!pending;
  useEffect(()=>onDirtyChange?.(dirty||uncertain||phase==='preflight'),[dirty,uncertain,phase,onDirtyChange]);
  useEffect(()=>{if(!dirty&&!uncertain)return;const handler=(event:BeforeUnloadEvent)=>event.preventDefault();
    window.addEventListener('beforeunload',handler);return()=>window.removeEventListener('beforeunload',handler);},[dirty,uncertain]);
  const locked=phase==='preflight'||phase==='saving'||phase==='unconfirmed';
  const editable=verified&&!!base?.capabilities.canManageDefinition&&!permissionRevoked&&!locked;
  const field=draft?.fields.find(item=>item.id===selected);
  const node=draft&&selected?findNode(draft.layout,selected):undefined;
  const fieldNode=draft&&field?findFieldNode(draft.layout,field.id):undefined;
  const availableGroups=draft?groupsIn(draft.layout):[];
  const unplacedFields=draft?draft.fields.filter(item=>!placedFields(draft.layout).has(item.id)):[];
  const patchField=useCallback((id:UUID,patch:Partial<FieldInput>)=>{
    setDraft(previous=>previous?{...previous,fields:previous.fields.map(item=>item.id===id?{...item,...patch} as FieldInput:item)}:previous);
    setNotice('');
  },[]);
  const patchNode=useCallback((id:UUID,patch:Partial<LayoutNodeInput>)=>{
    setDraft(previous=>previous?{...previous,layout:mapLayout(previous.layout,item=>item.id===id?{...item,...patch} as LayoutNodeInput:item)}:previous);
    setNotice('');
  },[]);
  const moveToGroup=(nodeId:UUID,target:UUID|null)=>{
    setDraft(previous=>{if(!previous)return previous;const item=findNode(previous.layout,nodeId);if(!item)return previous;
      if(target===item.id||(item.kind==='group'&&target&&contains(item.children,target)))return previous;
      const removed=removeNode(previous.layout,nodeId);
      return {...previous,layout:target?mapLayout(removed,entry=>entry.id===target&&entry.kind==='group'
        ?{...entry,children:[...entry.children,item]}:entry):[...removed,item]};});
    setGroup(target);setNotice('');
  };
  const addKind=useCallback((kind:PaletteKind,dropTarget?:UUID|null)=>{
    if(!editable)return;
    const systemCode=kind==='system_field'?base?.systemFields.find(item=>!systemCodesIn(draft?.layout??[]).includes(item.id))?.id:undefined;
    if(kind==='system_field'&&!systemCode){setError('所有系统字段都已添加到此视图');return;}
    const id=uuid(),created=dataKind(kind)?makeField(kind):null;
    const next:LayoutNodeInput=created?{id,kind:'field',fieldId:created.id,span:12}:
      kind==='group'?{id,kind:'group',title:'新建分组',span:12,children:[]}:
      kind==='divider'?{id,kind:'divider'}:kind==='description'?{id,kind:'description',text:'说明文字'}:
      {id,kind:'system_field',fieldId:systemCode!,span:12};
    setDraft(previous=>{
      if(!previous)return previous;
      const target=dropTarget===undefined?group:dropTarget;
      const parent=target?findNode(previous.layout,target):null;
      const layout=parent?.kind==='group'?mapLayout(previous.layout,item=>item.id===target&&item.kind==='group'
        ?{...item,children:[...item.children,next]}:item):
        target&&parent?insertAfter(previous.layout,target,next):[...previous.layout,next];
      return {...previous,fields:created?[...previous.fields,created]:previous.fields,layout};
    });
    setSelected(created?.id??id);setNotice('');
  },[editable,group,base,draft]);
  const addExistingField=(fieldId:UUID,dropTarget?:UUID|null)=>{
    if(!editable)return;
    const id=uuid();
    setDraft(previous=>{
      if(!previous||!previous.fields.some(item=>item.id===fieldId)||placedFields(previous.layout).has(fieldId))return previous;
      const next:LayoutNodeInput={id,kind:'field',fieldId,span:12};
      const target=dropTarget===undefined?group:dropTarget;
      const parent=target?findNode(previous.layout,target):null;
      const layout=parent?.kind==='group'?mapLayout(previous.layout,item=>item.id===target&&item.kind==='group'
        ?{...item,children:[...item.children,next]}:item):
        target&&parent?insertAfter(previous.layout,target,next):[...previous.layout,next];
      return {...previous,layout};
    });
    setSelected(fieldId);setNotice('');
  };
  const drop=(event:DragEvent<HTMLElement>,target?:UUID)=>{
    event.preventDefault();if(!editable)return;
    try{const value=JSON.parse(event.dataTransfer.getData('application/x-weaveos-form')) as
      {kind:'palette';value:PaletteKind}|{kind:'layout';value:UUID}|{kind:'existing';value:UUID};
      if(value.kind==='palette'){addKind(value.value,target??null);return;}
      if(value.kind==='existing'){addExistingField(value.value,target??null);return;}
      if(value.value===target)return;
      setDraft(previous=>{if(!previous)return previous;const item=findNode(previous.layout,value.value);
        if(!item)return previous;const removed=removeNode(previous.layout,item.id);
        if(target&&item.kind==='group'&&contains(item.children,target))return previous;
        const parent=target?findNode(removed,target):null;
        const layout=parent?.kind==='group'?mapLayout(removed,entry=>entry.id===target&&entry.kind==='group'
          ?{...entry,children:[...entry.children,item]}:entry):
          target&&parent?insertAfter(removed,target,item):[...removed,item];
        return {...previous,layout};});
    }catch{ /* unrelated browser drag */ }
  };
  const accepted=(value:{definition:Definition})=>{setBase(value.definition);setDraft(fromDefinition(value.definition));
    setPending(null);setImpact(null);setImpactStale(false);setImpactError('');setDialog(null);setPhase('idle');setError('');setNotice('已保存');};
  const cancelPreflight=()=>{saveEpoch.current++;preflightAbort.current?.abort();preflightAbort.current=null;};
  if(!leaveController.current)leaveController.current=createLeaveController(()=>{
    const {base:currentBase,draft:currentDraft,phase:currentPhase,pending:currentPending}=live.current;
    const status:LeaveStatus=currentPhase==='saving'?'write_in_flight':currentPhase==='unconfirmed'?'unknown':
      currentPhase==='preflight'?'preflight':currentPending||definitionDirty(currentBase,currentDraft)?'draft':'clean';
    return {status,fingerprint:JSON.stringify({status,phase:currentPhase,
      schemaVersion:currentBase?.table.schemaVersion,viewVersion:currentBase?.form.viewVersion,
      draft:currentDraft,pending:currentPending})};
  },decision=>{
    const cached=draftMemory.get(scopeKey);
    if(decision==='retain_operation'){
      if(cached)draftMemory.set(scopeKey,{...cached,base:live.current.base,draft:live.current.draft,
        phase:live.current.phase,pending:live.current.pending});
      setDialog(null);return;
    }
    cancelPreflight();
    const clean=live.current.base?fromDefinition(live.current.base):null;
    if(cached)draftMemory.set(scopeKey,{...cached,base:live.current.base,draft:clean,phase:'idle',pending:null,
      error:'',notice:'',selected:null,group:null});
    setDraft(clean);setPending(null);setPhase('idle');setImpact(null);setImpactStale(false);
    setImpactError('');setSelected(null);setGroup(null);setError('');setNotice('');setDialog(null);
  });
  useEffect(()=>registerLeaveGuard({kind:'designer',actorId,appId,viewId},leaveController.current!),
    [registerLeaveGuard,actorId,appId,viewId]);
  const openLeave=()=>{const status=leaveController.current!.getStatus();
    if(status==='clean'){onBack?.();return;}
    setLeavePrompt(status);setDialog('dirty');};
  const commit=async(input:SaveWrite,wasUnknown=false)=>{const current=scope.current;
    const cached=draftMemory.get(scopeKey);if(cached)draftMemory.set(scopeKey,{...cached,pending:input,phase:'saving'});
    setPending(input);setPhase('saving');setDialog(null);setError('');
    try{const saved=await formApi.save(appId,actorId,base!.table.id,viewId,input);if(!alive.current||scope.current!==current)return;accepted(saved);}
    catch(problem){if(!alive.current||scope.current!==current)return;
      reportAuth(problem);
      if(unknownWrite(problem)){setPhase('unconfirmed');setError('保存结果暂未确认，请查询原操作或按原请求重试');}
      else if(wasUnknown){setPhase('unconfirmed');setError(`原保存仍未确认：${formErrorText(problem)}`);
        if(problem instanceof FormApiError&&problem.status===403)setPermissionRevoked(true);}
      else{setPhase('idle');setPending(null);setError(formErrorText(problem));
        if(problem instanceof FormApiError&&problem.status===403)setPermissionRevoked(true);
        if(problem instanceof FormApiError&&problem.status===409&&problem.code!=='AUTH_SESSION_CHANGED')setDialog('conflict');}}
  };
  const save=async()=>{if(!draft||!editable)return;const current=scope.current,epoch=++saveEpoch.current;
    preflightAbort.current?.abort();const controller=new AbortController();preflightAbort.current=controller;
    setPhase('preflight');setError('');setNotice('');
    try{const plan=await formApi.preflight(appId,actorId,base.table.id,viewId,draft,controller.signal);
      if(!alive.current||scope.current!==current||epoch!==saveEpoch.current||controller.signal.aborted)return;
      preflightAbort.current=null;
      setImpact(plan);setImpactStale(false);setImpactError('');
      if(!plan.saveAllowed){setError(plan.dependencies.length?'字段仍被流程或其他视图引用，无法保存':
        '预检发现问题，请按提示调整字段或布局');setDialog('impact');setPhase('idle');return;}
      const input=immutablePacket<SaveWrite>({...draft,operationId:uuid(),confirmationToken:plan.confirmation?.token??null});
      if(plan.impacts.length){setPending(input);setDialog('impact');setPhase('idle');return;}
      await commit(input);
    }catch(problem){if(!alive.current||scope.current!==current||epoch!==saveEpoch.current||controller.signal.aborted)return;
      preflightAbort.current=null;reportAuth(problem);
      setPhase('idle');setError(formErrorText(problem));
      if(problem instanceof FormApiError&&problem.status===403)setPermissionRevoked(true);}
  };
  const checkOriginal=async()=>{if(!pending)return;const current=scope.current;setError('正在查询原操作结果…');
    try{const operation=await formApi.operation(pending.operationId,actorId);
      if(!alive.current||scope.current!==current)return;
      if(operation.httpStatus===200&&base&&validSaveResult(operation.result,appId,base.table.id,viewId,pending)){
        accepted(operation.result);return;}
      setError('原操作回执与当前表单或原请求不匹配；请保留草稿');
    }catch(problem){if(!alive.current||scope.current!==current)return;reportAuth(problem);
      if(problem instanceof FormApiError&&problem.status===403)setPermissionRevoked(true);
      setError(problem instanceof FormApiError&&problem.status===404?
      '暂未查到原操作；这不能证明保存已回滚，可按原请求重试':formErrorText(problem));}
  };
  if(!verified)return <section className="forms-module forms-loading" role="status">
    {verificationError?<><p role="alert">{verificationError}</p>
      <button type="button" onClick={()=>{setVerificationError('');setReload(value=>value+1);}}>重试</button></>:
      '正在验证表单访问权限…'}</section>;
  if(phase==='loading')return <section className="forms-module forms-loading" role="status">正在加载表单定义…</section>;
  if(!base||!draft)return <section className="forms-module forms-loading"><p role="alert">{error||'无法读取表单定义'}</p>
    <button type="button" onClick={()=>setReload(value=>value+1)}>重试</button></section>;
  const canvasNode=(item:LayoutNodeInput):React.ReactNode=>{
    const current=item.kind==='field'?draft.fields.find(entry=>entry.id===item.fieldId):null;
    const label=current?.name??(item.kind==='group'?item.title:item.kind==='description'?item.text:
      item.kind==='system_field'?`系统字段 · ${item.fieldId}`:'分割线');
    const active=selected===(current?.id??item.id);
    const span=item.kind==='field'||item.kind==='system_field'||item.kind==='group'?item.span??12:12;
    return <div key={item.id} className={`forms-canvas-node${active?' selected':''}`}
      style={{gridColumn:`span ${span} / span ${span}`}} draggable={editable}
      onDragStart={event=>event.dataTransfer.setData('application/x-weaveos-form',JSON.stringify({kind:'layout',value:item.id}))}
      onDragOver={event=>event.preventDefault()} onDrop={event=>{event.stopPropagation();drop(event,item.id);}}>
      <button type="button" className="forms-node-select" aria-pressed={active}
        onClick={()=>{setSelected(current?.id??item.id);setGroup(item.kind==='group'?item.id:null);}}>
        <span aria-hidden="true">⋮⋮</span><span>{label}{current?.required?' *':''}</span>
        <small>{current?labelFor[current.kind]:item.kind==='group'?'分组':''}</small>
      </button>
      {current&&<div className="forms-canvas-control"><FieldRenderer field={current} idPrefix="canvas"
        value={current.default} readOnly/></div>}
      {editable&&<div className="forms-node-actions">
        <button type="button" aria-label={`上移 ${label}`} onClick={()=>setDraft(previous=>previous?{...previous,layout:moveNode(previous.layout,item.id,-1)}:previous)}>上移</button>
        <button type="button" aria-label={`下移 ${label}`} onClick={()=>setDraft(previous=>previous?{...previous,layout:moveNode(previous.layout,item.id,1)}:previous)}>下移</button>
        {item.kind!=='divider'&&item.kind!=='description'&&<>
          <button type="button" aria-label={`缩小 ${label}`} disabled={span<=1} onClick={()=>patchNode(item.id,{span:span-1})}>−</button>
          <button type="button" aria-label={`扩大 ${label}`} disabled={span>=12} onClick={()=>patchNode(item.id,{span:span+1})}>＋</button>
        </>}
      </div>}
      {item.kind==='group'&&<div className="forms-canvas-grid forms-nested-grid"
        onDragOver={event=>event.preventDefault()} onDrop={event=>{event.stopPropagation();drop(event,item.id);}}>
        {item.children.map(canvasNode)}{!item.children.length&&<p>选择分组后从左侧添加字段</p>}
      </div>}
    </div>;
  };
  return <section className="forms-module" aria-label="表单设计器">
    <div className="forms-toolbar"><div className="forms-toolbar-title">
      <button type="button" className="forms-link" onClick={openLeave}>返回工作台</button>
      <strong>{base.form.name}</strong>{dirty&&<span className="forms-unsaved">未保存</span>}</div>
      <div className="forms-toolbar-actions">
        {(phase==='preflight'||phase==='saving')&&<span role="status">{phase==='preflight'?'正在预检…':'正在保存…'}</span>}
        {phase==='unconfirmed'&&<><button type="button" onClick={()=>void checkOriginal()}>查询保存结果</button>
          <button type="button" disabled={permissionRevoked} onClick={()=>pending&&void commit(pending,true)}>按原请求重试</button></>}
        <button type="button" onClick={()=>setDialog('preview')}>预览</button>
        <button type="button" className="forms-primary" disabled={!editable} onClick={()=>void save()}>保存</button>
      </div></div>
    {error&&<p className="forms-alert" role="alert">{error}</p>}
    {notice&&<p className="forms-success" role="status">{notice}</p>}
    {(!base.capabilities.canManageDefinition||permissionRevoked)&&!error&&<p className="forms-alert" role="alert">没有表单配置权限，当前仅可查看</p>}
    <div className="forms-designer-grid">
      <section className="forms-panel forms-palette" role="region" aria-label="字段面板"><h2>字段面板</h2>
        <div>{palette.map(entry=><button type="button" key={entry.kind} disabled={!editable}
          draggable={editable} onDragStart={event=>event.dataTransfer.setData('application/x-weaveos-form',JSON.stringify({kind:'palette',value:entry.kind}))}
          onClick={()=>addKind(entry.kind)}>{entry.label}</button>)}</div>
        {!!unplacedFields.length&&<><h3>已有字段</h3><div>{unplacedFields.map(item=><button type="button"
          key={item.id} aria-label={`将已有字段加入布局 ${item.name}`} disabled={!editable} draggable={editable}
          onDragStart={event=>event.dataTransfer.setData('application/x-weaveos-form',
            JSON.stringify({kind:'existing',value:item.id}))}
          onClick={()=>addExistingField(item.id)}>{item.name} · {labelFor[item.kind]}</button>)}</div></>}
        <h3>高级字段</h3><p>附件、关联、公式等将在后续版本接入。</p>
      </section>
      <section className="forms-panel forms-canvas" role="region" aria-label="表单画布"
        onDragOver={event=>event.preventDefault()} onDrop={event=>drop(event)}>
        <h2>{base.form.name}</h2><p className="forms-muted">拖拽或用键盘添加字段，保存后才会生效</p>
        <div className="forms-canvas-grid">{draft.layout.map(canvasNode)}
          {!draft.layout.length&&<div className="forms-canvas-empty">从左侧选择字段，或拖入这里开始设计表单</div>}</div>
      </section>
      <section className="forms-panel forms-properties" role="region" aria-label="字段属性"><h2>字段属性</h2>
        {field?<>
          <h3>{field.name}</h3><label>稳定字段 ID<input value={field.id} readOnly/></label>
          <label>字段名称<input aria-label="字段名称" value={field.name} disabled={!editable}
            onChange={event=>patchField(field.id,{name:event.target.value})}/></label>
          <label>字段类型<select aria-label="字段类型" value={field.kind} disabled={!editable}
            onChange={event=>{const kind=event.target.value as FieldKind;
              setDraft(previous=>previous?{...previous,fields:previous.fields.map(item=>item.id===field.id&&item.kind!==kind
                ?{...makeField(kind),id:item.id,name:item.name,required:item.required,
                  presentation:{...item.presentation,displayTimeZone:kind==='datetime'&&item.kind==='datetime'
                    ?item.presentation.displayTimeZone:null},default:null}:item)}:previous);
              setNotice('');}}>
            {palette.filter(entry=>dataKind(entry.kind)).map(entry=><option key={entry.kind}
              value={entry.kind}>{entry.label}</option>)}</select></label>
          <label className="forms-checkbox"><input type="checkbox" checked={field.required} disabled={!editable}
            onChange={event=>patchField(field.id,{required:event.target.checked})}/>必填</label>
          {fieldNode&&<label>字段宽度<select aria-label="字段宽度" value={fieldNode.span??12} disabled={!editable}
            onChange={event=>patchNode(fieldNode.id,{span:Number(event.target.value)})}>
            {Array.from({length:12},(_,index)=>index+1).map(value=><option key={value} value={value}>{value} / 12</option>)}</select></label>}
          {fieldNode&&<label>所属分组<select aria-label="所属分组" value={parentGroup(draft.layout,fieldNode.id)??''} disabled={!editable}
            onChange={event=>moveToGroup(fieldNode.id,event.target.value||null)}>
            <option value="">画布根层</option>{availableGroups.map(item=><option key={item.id} value={item.id}>{item.title}</option>)}
          </select></label>}
          <label>帮助文字<input value={field.presentation.helpText??''} disabled={!editable}
            onChange={event=>patchField(field.id,{presentation:{...field.presentation,helpText:event.target.value||null}})}/></label>
          <FieldConfig key={`${field.id}/${field.kind}`} appId={appId} actorId={actorId} field={field}
            onAuthError={reportAuth}
            disabled={!editable} onPatch={patch=>patchField(field.id,patch)}/>
          <button type="button" className="forms-danger" disabled={!editable} onClick={()=>{
            setDraft(previous=>previous?{...previous,fields:previous.fields.filter(item=>item.id!==field.id),
              layout:fieldNode?removeNode(previous.layout,fieldNode.id):previous.layout}:previous);setSelected(null);
          }}>删除字段</button>
        </>:node?<>
          {node.kind==='group'&&<label>分组标题<input value={node.title} disabled={!editable}
            onChange={event=>patchNode(node.id,{title:event.target.value})}/></label>}
          {node.kind==='description'&&<label>说明文字<textarea value={node.text} disabled={!editable}
            onChange={event=>patchNode(node.id,{text:event.target.value})}/></label>}
          {node.kind==='system_field'&&<label>只读系统字段<select value={node.fieldId} disabled={!editable}
            onChange={event=>patchNode(node.id,{fieldId:event.target.value as typeof node.fieldId})}>
            {base.systemFields.filter(item=>item.id===node.fieldId||!systemCodesIn(draft.layout).includes(item.id))
              .map(item=><option key={item.id} value={item.id}>{item.id}</option>)}</select></label>}
          {(node.kind==='group'||node.kind==='system_field')&&<label>宽度<select value={node.span??12} disabled={!editable}
            onChange={event=>patchNode(node.id,{span:Number(event.target.value)})}>
            {Array.from({length:12},(_,index)=>index+1).map(value=><option key={value} value={value}>{value} / 12</option>)}</select></label>}
          {(node.kind==='group'||node.kind==='system_field')&&<label>所属分组<select aria-label="所属分组" value={parentGroup(draft.layout,node.id)??''} disabled={!editable}
            onChange={event=>moveToGroup(node.id,event.target.value||null)}><option value="">画布根层</option>
            {availableGroups.filter(item=>item.id!==node.id&&
              (node.kind!=='group'||!contains(node.children,item.id)))
              .map(item=><option key={item.id} value={item.id}>{item.title}</option>)}</select></label>}
          <button type="button" className="forms-danger" disabled={!editable}
            onClick={()=>{setDraft(previous=>previous?{...previous,layout:removeNode(previous.layout,node.id)}:previous);setSelected(null);}}>移除布局节点</button>
        </>:<p className="forms-muted">选择画布中的字段，配置名称、校验、默认值和宽度。</p>}
      </section>
    </div>
    {dialog==='preview'&&<FormsDialog title="表单预览" onClose={()=>setDialog(null)}>
      <p className="forms-muted">本地预览，尚未保存。输入不会创建记录。</p>
      <div className="forms-preview"><FormPreview fields={draft.fields} layout={draft.layout}/></div>
      <div className="forms-dialog-actions"><button type="button" data-forms-close>返回设计器</button></div>
    </FormsDialog>}
    {dialog==='impact'&&<FormsDialog title="保存预检：表单结构与布局" onClose={()=>setDialog(null)}>
      {impactStale&&<p className="forms-alert" role="alert">{impactError||'当前确认方案已失效，请重新预检'}</p>}
      <p>本次变更计划</p><ul>{impact?.plan.schemaChanges.map(change=><li key={change.fieldId+change.kind}>{change.kind} · {change.fieldId}</li>)}
        {impact?.plan.layoutChanged&&<li>表单布局将更新</li>}</ul>
      {!!impact?.impacts.length&&<><h3>数据影响</h3><ul>{impact.impacts.map(item=><li key={item.fieldId+item.kind}>
        字段 {item.fieldId} · {item.nonNullRows} 条已有值</li>)}</ul></>}
      {impact?.impacts.filter(item=>item.kind==='option_mapping'&&item.optionId).map(item=>{
        const previous=base.fields.find(entry=>entry.id===item.fieldId);
        const current=draft.fields.find(entry=>entry.id===item.fieldId);
        const oldLabel=previous&&(previous.kind==='single_select'||previous.kind==='multi_select')
          ?previous.config.options.find(option=>option.id===item.optionId)?.label??item.optionId:item.optionId;
        const targets=current&&(current.kind==='single_select'||current.kind==='multi_select')?current.config.options:[];
        const mapping=(pending?.optionMappings??draft.optionMappings).find(entry=>entry.fieldId===item.fieldId&&entry.fromOptionId===item.optionId);
        return <label key={item.fieldId+item.optionId}>已用选项映射 {oldLabel}
          <select aria-label={`已用选项映射 ${oldLabel}`} value={mapping?mapping.toOptionId??'__null__':''}
            disabled={phase==='preflight'} onChange={event=>{
              const target=event.target.value==='__null__'?null:event.target.value;
              if(pending&&mapping?.toOptionId!==target){setPending(null);setImpactStale(true);
                setImpactError('映射已修改，请重新预检');}
              setDraft(previousDraft=>previousDraft?{
                ...previousDraft,optionMappings:[
                  ...previousDraft.optionMappings.filter(entry=>!(entry.fieldId===item.fieldId&&entry.fromOptionId===item.optionId)),
                  {fieldId:item.fieldId,fromOptionId:item.optionId!,toOptionId:target},
                ],
              }:previousDraft);
            }}>
            <option value="">请选择处理方式</option><option value="__null__">清空已有值</option>
            {targets.map(option=><option key={option.id} value={option.id}>{option.label}</option>)}
          </select></label>;
      })}
      {!!impact?.dependencies.length&&<><h3>阻止保存的依赖</h3><ul>{impact.dependencies.map(item=><li key={item.fieldId+item.resourceId}>
        {item.kind} · {item.resourceId}</li>)}</ul></>}
      {!!impact?.blockingIssues.length&&<ul>{impact.blockingIssues.map(item=><li key={item.code}>{item.code} · {item.fieldIds.join('、')}</li>)}</ul>}
      {impact?.confirmation?<p>确认有效期至 {new Date(impact.confirmation.expiresAt).toLocaleString('zh-CN')}</p>:
        impact?.impacts.length?<p className="forms-alert" role="alert">预检未返回有效确认令牌，请重新预检后再保存。</p>:null}
      <div className="forms-dialog-actions"><button type="button" data-forms-close>继续编辑</button>
        {(impactStale||impact?.blockingIssues.some(item=>item.code==='APPLICATION_SCHEMA_OPTION_MAPPING_REQUIRED'))&&
          <button type="button" disabled={phase==='preflight'} onClick={()=>{setDialog(null);void save();}}>重新预检</button>}
        {impact?.saveAllowed&&<button type="button" className="forms-primary"
          disabled={!pending||impactStale||!impact.confirmation?.token}
          onClick={()=>{
            if(!pending||!impact.confirmation?.token)return;
            if(Date.parse(impact.confirmation.expiresAt)<=Date.now()){
              setPending(null);setImpactStale(true);setImpactError('确认已过期，请重新预检');return;
            }
            void commit(pending);
          }}>确认保存</button>}</div>
    </FormsDialog>}
    {dialog==='dirty'&&<FormsDialog title={leavePrompt==='unknown'||leavePrompt==='write_in_flight'?'保存结果未确认':'结构或布局尚未保存'} onClose={()=>setDialog(null)}>
      <p>{leavePrompt==='unknown'||leavePrompt==='write_in_flight'?'离开前请查询原操作结果；当前草稿和原操作号将在此浏览器页面中保留。':'离开将放弃此处未保存的修改。'}</p><div className="forms-dialog-actions">
        <button type="button" data-forms-close>继续编辑</button>
        <button type="button" className="forms-danger" onClick={()=>{
          const decision=leavePrompt==='unknown'||leavePrompt==='write_in_flight'?'retain_operation':'discard';
          const result=leaveController.current!.prepareLeave(decision);
          if(!result.ok){setLeavePrompt(leaveController.current!.getStatus());
            if(result.status==='clean')setDialog(null);return;}
          onBack?.();}}>
          {leavePrompt==='unknown'||leavePrompt==='write_in_flight'?'离开并保留待核查操作':'放弃修改并离开'}</button>
      </div></FormsDialog>}
    {dialog==='conflict'&&<FormsDialog title="配置版本已变化" onClose={()=>setDialog(null)}>
      <p>其他编辑者已修改此表单。当前输入仍保留，请核对服务器最新定义。</p>
      <div className="forms-dialog-actions"><button type="button" data-forms-close>继续查看我的输入</button>
        <button type="button" onClick={()=>{
          draftMemory.delete(scopeKey);
          setVerified(false);setBase(null);setDraft(null);setPhase('loading');
          setPending(null);setImpact(null);setSelected(null);setGroup(null);
          setError('');setNotice('');setDialog(null);setReload(value=>value+1);
        }}>放弃草稿并加载最新版</button></div>
    </FormsDialog>}
  </section>;
}

function findFieldNode(nodes:LayoutNodeInput[],fieldId:UUID):Extract<LayoutNodeInput,{kind:'field'}>|undefined {
  for(const node of nodes){if(node.kind==='field'&&node.fieldId===fieldId)return node;
    if(node.kind==='group'){const nested=findFieldNode(node.children,fieldId);if(nested)return nested;}}
}

function FieldConfig({appId,actorId,field,disabled,onPatch,onAuthError}:{appId:UUID;actorId:UUID;
  field:FieldInput;disabled:boolean;onPatch:(patch:Partial<FieldInput>)=>void;
  onAuthError:(problem:unknown)=>void}){
  if(field.kind==='number'||field.kind==='money'){
    const cfg=field.config,change=(patch:object)=>onPatch({config:{...cfg,...patch}} as Partial<FieldInput>);
    return <>
      <label>总精度<input type="number" min="1" max="38" value={cfg.precision??38} disabled={disabled}
        onChange={event=>change({precision:Number(event.target.value)})}/></label>
      <label>小数位数<input type="number" min="0" max="18" value={cfg.scale??(field.kind==='money'?2:0)} disabled={disabled}
        onChange={event=>change({scale:Number(event.target.value)})}/></label>
      <label>处理位数<input type="number" min="-18" max="18" value={cfg.roundingPlaces??cfg.scale??(field.kind==='money'?2:0)} disabled={disabled}
        onChange={event=>change({roundingPlaces:Number(event.target.value)})}/></label>
      <label>舍入规则<select value={cfg.roundingMode??'HALF_UP'} disabled={disabled}
        onChange={event=>change({roundingMode:event.target.value})}>
        {['HALF_UP','HALF_EVEN','TOWARD_ZERO','FLOOR','CEILING'].map(mode=><option key={mode}>{mode}</option>)}</select></label>
      <p className="forms-muted">负处理位数按十、百等整数位置舍入；数字以十进制字符串传输。</p>
      <label>默认值<input inputMode="decimal" value={field.default??''} disabled={disabled}
        onChange={event=>onPatch({default:event.target.value||null} as Partial<FieldInput>)}/></label>
    </>;
  }
  if(field.kind==='text'||field.kind==='multiline')return <>
    <label>最大字数<input type="number" min="1" value={field.config.maxLength??''} disabled={disabled}
      onChange={event=>onPatch({config:{maxLength:event.target.value?Number(event.target.value):null}} as Partial<FieldInput>)}/></label>
    <label className="forms-checkbox"><input type="checkbox" aria-label="启用默认值" checked={field.default!==null}
      disabled={disabled} onChange={event=>onPatch({default:event.target.checked?'':null} as Partial<FieldInput>)}/>启用默认值</label>
    {field.default!==null&&<label>默认值<input value={field.default} disabled={disabled}
      onChange={event=>onPatch({default:event.target.value} as Partial<FieldInput>)}/></label>}
  </>;
  if(field.kind==='datetime')return <>
    <label>时间精度<select value={field.config.precision??'second'} disabled={disabled}
      onChange={event=>onPatch({config:{precision:event.target.value as 'minute'|'second'|'millisecond'}} as Partial<FieldInput>)}>
      <option value="minute">分钟</option><option value="second">秒</option><option value="millisecond">毫秒</option></select></label>
    <label>展示时区<input value={field.presentation.displayTimeZone??'UTC'} disabled={disabled}
      onChange={event=>onPatch({presentation:{...field.presentation,displayTimeZone:event.target.value||null}})}/></label>
    <label>默认值<input placeholder="含时区偏移的 RFC3339" value={field.default??''} disabled={disabled}
      onChange={event=>onPatch({default:event.target.value||null} as Partial<FieldInput>)}/></label>
  </>;
  if(field.kind==='single_select'||field.kind==='multi_select')return <>
    <h4>选项</h4>{field.config.options.map(option=><div className="forms-option-row" key={option.id}>
      <input aria-label={`选项 ${option.id}`} value={option.label} disabled={disabled}
        onChange={event=>onPatch({config:{options:field.config.options.map(item=>item.id===option.id?{...item,label:event.target.value}:item)}} as Partial<FieldInput>)}/>
      <button type="button" aria-label={`删除选项 ${option.label}`} disabled={disabled}
        onClick={()=>{const remaining=field.config.options.filter(item=>item.id!==option.id);
          const currentDefault=field.default;
          const nextDefault=Array.isArray(currentDefault)?currentDefault.filter(id=>id!==option.id):
            currentDefault===option.id?null:currentDefault;
          onPatch({config:{options:remaining},default:nextDefault} as Partial<FieldInput>);
        }}>×</button></div>)}
    <button type="button" disabled={disabled} onClick={()=>onPatch({config:{options:[...field.config.options,{id:uuid(),label:`选项 ${field.config.options.length+1}`}]}} as Partial<FieldInput>)}>添加选项</button>
    <small>选项 ID 改名不变，删除已使用选项需预检并明确映射。</small>
    {field.kind==='single_select'?<label>默认选项<select value={field.default??''} disabled={disabled}
      onChange={event=>onPatch({default:event.target.value||null} as Partial<FieldInput>)}>
      <option value="">无默认值</option>{field.config.options.map(item=><option key={item.id} value={item.id}>{item.label}</option>)}</select></label>:
      <label>默认选项<select multiple value={field.default??[]} disabled={disabled}
        onChange={event=>onPatch({default:Array.from(event.target.selectedOptions,item=>item.value)} as Partial<FieldInput>)}>
        {field.config.options.map(item=><option key={item.id} value={item.id}>{item.label}</option>)}</select></label>}
  </>;
  if(field.kind==='boolean')return <label>默认状态<select aria-label="默认状态"
    value={field.default===null?'unset':field.default?'true':'false'} disabled={disabled}
    onChange={event=>onPatch({default:event.target.value==='unset'?null:event.target.value==='true'} as Partial<FieldInput>)}>
    <option value="unset">无默认值</option><option value="false">默认不勾选</option><option value="true">默认勾选</option>
  </select></label>;
  if(field.kind==='member'||field.kind==='department')return <ReferenceDefaultSelector
    appId={appId} actorId={actorId} kind={field.kind} value={field.default} disabled={disabled}
    onAuthError={onAuthError}
    onChange={value=>onPatch({default:value} as Partial<FieldInput>)}/>;
  return <label>默认值<input value={field.default??''} disabled={disabled}
    onChange={event=>onPatch({default:event.target.value||null} as Partial<FieldInput>)}/></label>;
}

function ReferenceDefaultSelector({appId,actorId,kind,value,disabled,onChange,onAuthError}:{
  appId:UUID;actorId:UUID;kind:'member'|'department';value:UUID|null;disabled:boolean;
  onChange:(value:UUID|null)=>void;onAuthError:(problem:unknown)=>void;
}){
  const [search,setSearch]=useState(''),[query,setQuery]=useState('');
  const [items,setItems]=useState<ReferenceCandidate[]>([]),[next,setNext]=useState<string|null>(null);
  const [loading,setLoading]=useState(false),[error,setError]=useState('');
  const generation=useRef(0);
  useEffect(()=>{
    const controller=new AbortController(),current=++generation.current;
    setLoading(true);setError('');setItems([]);setNext(null);
    void formApi.candidates(appId,actorId,kind,query,undefined,controller.signal).then(page=>{
      if(current!==generation.current)return;
      setItems(page.items);setNext(page.nextPageToken);
    }).catch(problem=>{if(!controller.signal.aborted&&current===generation.current){
      onAuthError(problem);setError(formErrorText(problem));}})
      .finally(()=>{if(current===generation.current)setLoading(false);});
    return()=>{controller.abort();generation.current++;};
  },[appId,actorId,kind,query]);
  const more=async()=>{if(!next||loading)return;const current=++generation.current;
    setLoading(true);setError('');
    try{const page=await formApi.candidates(appId,actorId,kind,query,next);
      if(current===generation.current){setItems(previous=>[...previous,...page.items]);setNext(page.nextPageToken);}}
    catch(problem){if(current===generation.current){onAuthError(problem);setError(formErrorText(problem));}}
    finally{if(current===generation.current)setLoading(false);}
  };
  const label=kind==='member'?'成员':'部门';
  return <div className="forms-reference-default">
    {value&&<><label>当前默认引用 ID<input aria-label="当前默认引用 ID" value={value} readOnly/></label>
      <button type="button" disabled={disabled} onClick={()=>onChange(null)}>清除默认引用</button></>}
    <label>搜索{label}<input aria-label={`搜索${label}`} value={search} disabled={disabled}
      onChange={event=>setSearch(event.target.value)} onKeyDown={event=>{if(event.key==='Enter')setQuery(search.trim());}}/></label>
    <button type="button" disabled={disabled||loading} onClick={()=>setQuery(search.trim())}>查询{label}</button>
    {loading&&<p role="status">正在读取{label}候选…</p>}
    {error&&<p role="alert">{label}候选读取失败：{error}</p>}
    {!loading&&!error&&!items.length&&<p className="forms-muted">没有匹配的{label}候选</p>}
    {!!items.length&&<label>默认{label}<select aria-label={`默认${label}`} value={value??''} disabled={disabled}
      onChange={event=>onChange(event.target.value||null)}>
      <option value="">无默认值</option>
      {value&&!items.some(item=>item.id===value)&&<option value={value}>当前引用 · {value}</option>}
      {items.map(item=><option key={item.id} value={item.id}>{item.label}</option>)}
    </select></label>}
    {next&&<button type="button" disabled={disabled||loading} onClick={()=>void more()}>加载更多{label}</button>}
    <p className="forms-muted">保存时服务端会重新检查默认引用是否仍可用。</p>
  </div>;
}
