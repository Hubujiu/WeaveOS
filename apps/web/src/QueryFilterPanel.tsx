import { startTransition, useId, useLayoutEffect, useRef, useState, ViewTransition } from 'react';
import { Popover } from '@base-ui/react/popover';
import { Funnel, X } from '@phosphor-icons/react';
import {
  eventFilterFields, memberFilterFields, filterOperators, MAX_FILTER_DEPTH,
  MAX_FILTER_LEAVES, validateQueryFilter, type FilterField, type QueryFilter, type QueryView,
} from './QueryFilterState';
import './query-filter.css';

export type FilterChoice = { value: string; label: string };
export type QueryFilterPanelProps<V extends QueryView> = {
  view: V;
  value?: QueryFilter<V>;
  onApply: (filter: QueryFilter<V> | undefined) => void;
  options?: { departmentIds?: readonly FilterChoice[]; identityIds?: readonly FilterChoice[] };
};

type EditableCondition = { id: string; kind: 'condition'; field: string; operator: string; value: unknown };
type EditableGroup = { id: string; kind: 'group'; operator: string; children: EditableNode[] };
type EditableNode = EditableCondition | EditableGroup;
const isRecord=(value:unknown):value is Record<string,unknown>=>typeof value==='object' && value!==null && !Array.isArray(value);
function editTree(value:unknown):EditableNode {
  const node=isRecord(value)?value:{};
  if(Array.isArray(node.children))return {id:crypto.randomUUID(),kind:'group',operator:String(node.operator),children:node.children.map(editTree)};
  return {id:crypto.randomUUID(),kind:'condition',field:String(node.field??''),operator:String(node.operator??'eq'),value:node.value};
}
function initialTree(value:unknown):EditableGroup {
  const node=editTree(value??{operator:'and',children:[]});
  return node.kind==='group'?node:{id:crypto.randomUUID(),kind:'group',operator:'and',children:[node]};
}
function wireTree(node:EditableNode):unknown {
  return node.kind==='group'?{operator:node.operator,children:node.children.map(wireTree)}:{field:node.field,operator:node.operator,value:node.value};
}
function countLeaves(node:EditableNode):number{return node.kind==='condition'?1:node.children.reduce((sum,child)=>sum+countLeaves(child),0);}

export function QueryFilterPanel<V extends QueryView>({view,value,onApply,options}: QueryFilterPanelProps<V>) {
  const [open,setOpen]=useState(false);
  const [draft,setDraft]=useState(()=>initialTree(value));
  const [popup,setPopup]=useState<HTMLDivElement|null>(null);
  const [reducedMotion,setReducedMotion]=useState(()=>typeof matchMedia!=='undefined'&&matchMedia('(prefers-reduced-motion: reduce)').matches);
  const trigger=useRef<HTMLButtonElement>(null);
  const firstControl=useRef<HTMLSelectElement>(null);
  const actions=useRef<Popover.Root.Actions|null>(null);
  const intent=useRef(false);
  const generation=useRef(0);
  const animation=useRef<Animation[]>([]);
  const panelId=useId();
  const sharedName=`q36-filter-shell-${panelId.replace(/[^a-zA-Z0-9_-]/g,'-')}`;
  // WebKit 26.6 can crash the page when a nested filter edit follows a native
  // shared transition. Keep its stable WAAPI path until that engine is safe.
  const sharedMotion=!reducedMotion&&typeof document!=='undefined'&&typeof document.startViewTransition==='function'
    && navigator.vendor!=='Apple Computer, Inc.';
  const fields=view==='members'?memberFilterFields:eventFilterFields;
  const validation=validateQueryFilter(view,wireTree(draft));
  const leafCount=countLeaves(draft);
  function changeOpen(next:boolean) {
    intent.current=next;
    if(sharedMotion) {
      // React owns the native transition; the shell's two named boundaries
      // exchange in one Transition. The table outside them stays stationary.
      document.documentElement.classList.add('q36-filter-transition-active');
      startTransition(()=>{
        if(next&&!open)setDraft(initialTree(value));
        setOpen(next);
      });
    } else {
      if(next&&!open)setDraft(initialTree(value));
      setOpen(next);
    }
  }
  useLayoutEffect(()=>{
    const media=matchMedia('(prefers-reduced-motion: reduce)');
    const sync=()=>setReducedMotion(media.matches);
    media.addEventListener('change',sync);
    return()=>media.removeEventListener('change',sync);
  },[]);
  // Independent shell/content layers avoid stretching glyphs. This WAAPI path
  // also remains the no-ViewTransition browser fallback after integration.
  useLayoutEffect(()=>{
    if(!popup)return;
    const version=++generation.current;
    if(sharedMotion){
      if(!open){
        const timer=window.setTimeout(()=>{
          if(version===generation.current&&!intent.current)actions.current?.unmount();
        },220);
        return()=>{generation.current++;window.clearTimeout(timer);};
      }
      return()=>{generation.current++;};
    }
    // The positioner computes its geometry after the first layout. Measure on
    // the next frame so a newly opened fallback popup has a nonzero target.
    const frame=requestAnimationFrame(()=>{
      if(version!==generation.current)return;
      const shell=popup.querySelector<HTMLElement>('.q36-filter-shell');
      const content=popup.querySelector<HTMLElement>('.q36-filter-content');
      const previous=animation.current;
      const currentShell=shell?getComputedStyle(shell).transform:'none';
      const currentOpacity=content?getComputedStyle(content).opacity:'0';
      previous.forEach(a=>a.cancel()); animation.current=[];
      const reduce=matchMedia('(prefers-reduced-motion: reduce)').matches;
      const complete=()=>{if(version===generation.current&&!intent.current)actions.current?.unmount();};
      if(reduce||typeof shell?.animate!=='function'){complete();return;}
      const from=trigger.current?.getBoundingClientRect();const to=popup.getBoundingClientRect();
      if(!from||!to.width||!to.height){complete();return;}
      const small=`translate(${from.left-to.left}px,${from.top-to.top}px) scale(${from.width/to.width},${from.height/to.height})`;
      const duration=open?300:220;
      const shellMotion=shell.animate([{transform:previous.length?currentShell:open?small:'none'},{transform:open?'none':small}],{duration,easing:'cubic-bezier(.2,.8,.2,1)',fill:'both'});
      animation.current.push(shellMotion);
      if(content)animation.current.push(content.animate([{opacity:previous.length?currentOpacity:open?0:1,transform:open?'translateY(-4px)':'none'},{opacity:open?1:0,transform:open?'none':'translateY(-4px)'}],{duration:open?160:80,delay:open?100:0,fill:'both',easing:'ease-out'}));
      shellMotion.finished.then(complete,()=>{});
    });
    return()=>{generation.current++;cancelAnimationFrame(frame);};
  },[open,popup,sharedMotion]);
  useLayoutEffect(()=>()=>animation.current.forEach(a=>a.cancel()),[]);
  function update(id:string,transform:(node:EditableNode)=>EditableNode) {
    function visit(node:EditableNode):EditableNode {
      if(node.id===id)return transform(node);
      return node.kind==='group'?{...node,children:node.children.map(visit)}:node;
    }
    setDraft(previous=>visit(previous) as EditableGroup);
  }
  function choices(field:FilterField|undefined) {
    return field?.key==='departmentIds'?options?.departmentIds:field?.key==='identityIds'?options?.identityIds:field?.choices;
  }
  function leaf(node:EditableCondition,path:string,issuePath:string) {
    const field=fields.find(f=>f.key===node.field);
    const allowed=field?.kind==='time'?filterOperators:filterOperators.slice(0,2);
    const invalid=validation.issues.some(i=>i.path===issuePath);
    const patch=(change:Partial<EditableCondition>)=>update(node.id,current=>current.kind==='condition'?{...current,...change}:current);
    const calendarValue=isRecord(node.value)?node.value:undefined;
    const calendar=calendarValue!==undefined;
    const mode=node.value===null?'null':calendar?'date':'value';
    const changeMode=(next:string)=>patch({value:next==='null'?null:next==='date'?{date:'',timeZone:Intl.DateTimeFormat().resolvedOptions().timeZone}:field?.kind==='boolean'?true:''});
    return <div className="q36-filter-condition" key={node.id} aria-invalid={invalid}>
      <select aria-label={`条件 ${path} 字段`} value={node.field} onChange={e=>{
        const next=fields.find(f=>f.key===e.target.value);
        patch({field:e.target.value,operator:'eq',value:next?.kind==='boolean'?true:''});
      }}><option value="">请选择字段</option>{!field&&node.field&&<option value={node.field}>未知字段</option>}{fields.map(f=><option key={f.key} value={f.key}>{f.label}</option>)}</select>
      <select aria-label={`条件 ${path} 比较`} value={node.operator} onChange={e=>patch({operator:e.target.value})}>{allowed.map(o=><option key={o.value} value={o.value}>{o.label}</option>)}</select>
      {field?.kind!=='relation'&&<select className="q36-filter-value-mode" aria-label={`条件 ${path} 值类型`} value={mode} onChange={e=>changeMode(e.target.value)}>
        <option value="value">{field?.kind==='time'?'绝对时刻':'值'}</option>{field?.kind==='time'&&<option value="date">日期及时区</option>}
        {(node.operator==='eq'||node.operator==='neq')&&<option value="null">NULL</option>}
      </select>}
      {mode==='null'?<span className="q36-filter-null">{node.operator==='neq'?'不为空':'为空'}</span>:field?.kind==='enum'||field?.kind==='relation'?<select aria-label={`条件 ${path} 值`} aria-invalid={invalid} value={typeof node.value==='string'?node.value:''} onChange={e=>patch({value:e.target.value})}>
        <option value="">请选择</option>{choices(field)?.map(c=><option key={c.value} value={c.value}>{c.label}</option>)}
      </select>:field?.kind==='boolean'?<select aria-label={`条件 ${path} 值`} value={String(node.value)} onChange={e=>patch({value:e.target.value==='true'})}><option value="true">是</option><option value="false">否</option></select>
        :calendarValue?<div className="q36-filter-date"><input aria-label={`条件 ${path} 日期`} type="date" value={String(calendarValue.date??'')} onChange={e=>patch({value:{...calendarValue,date:e.target.value}})} /><input aria-label={`条件 ${path} 时区`} placeholder="Asia/Shanghai" value={String(calendarValue.timeZone??'')} onChange={e=>patch({value:{...calendarValue,timeZone:e.target.value}})} /></div>
        :<input aria-label={`条件 ${path} 值`} aria-invalid={invalid} value={typeof node.value==='string'?node.value:''} placeholder={field?.kind==='time'?'2026-10-01T10:00:00+08:00':'精确值，区分大小写'} onChange={e=>patch({value:e.target.value})} />}
      <button type="button" className="q36-filter-delete" aria-label={`删除条件 ${path}`} onClick={()=>remove(node.id)}><X size={14}/></button>
    </div>;
  }
  function remove(id:string) {
    function visit(node:EditableGroup):EditableGroup{return {...node,children:node.children.filter(child=>child.id!==id).map(child=>child.kind==='group'?visit(child):child)};}
    setDraft(previous=>visit(previous));
  }
  function group(node:EditableGroup,path:string,depth:number,issuePath:string):React.ReactNode {
    return <fieldset className="q36-filter-group" key={node.id}>
      <legend>{depth===1?'筛选条件':`分组 ${path}`}</legend>
      <div className="q36-filter-group-header"><select ref={depth===1?firstControl:undefined} aria-label={`组 ${path} 匹配方式`} value={node.operator} onChange={e=>update(node.id,current=>({...current,operator:e.target.value}))}>
        <option value="and">全部条件（AND）</option><option value="or">任一条件（OR）</option>
      </select>{depth>1&&<button type="button" className="q36-filter-delete" aria-label={`删除组 ${path}`} onClick={()=>remove(node.id)}><X size={14}/></button>}</div>
      {node.children.map((child,index)=>child.kind==='group'?group(child,`${path}.${index+1}`,depth+1,`${issuePath}.${index}`):leaf(child,depth===1?String(index+1):`${path.slice(2)}.${index+1}`,`${issuePath}.${index}`))}
      <div className="q36-filter-group-actions">
        <button type="button" disabled={leafCount>=MAX_FILTER_LEAVES} aria-label={`组 ${path} 添加条件`} onClick={()=>update(node.id,current=>current.kind==='group'?{...current,children:[...current.children,{id:crypto.randomUUID(),kind:'condition',field:fields[0].key,operator:'eq',value:''}]}:current)}>＋ 条件</button>
        <button type="button" disabled={depth>=MAX_FILTER_DEPTH||leafCount>=MAX_FILTER_LEAVES} aria-label={`组 ${path} 添加分组`} onClick={()=>update(node.id,current=>current.kind==='group'?{...current,children:[...current.children,{id:crypto.randomUUID(),kind:'group',operator:'and',children:[]}]}:current)}>＋ 分组</button>
      </div>
    </fieldset>;
  }
  const content=<div className="q36-filter-content">
          <div className="q36-filter-heading"><Popover.Title>自定义筛选</Popover.Title><Popover.Close className="q36-filter-delete" aria-label="关闭筛选"><X size={16}/></Popover.Close></div>
          <Popover.Description>搜索、快捷条件与本筛选共同生效。最多 3 层分组、20 个条件。</Popover.Description>
          <div className="q36-filter-tree">{group(draft,'1',1,'root')}</div>
          {validation.issues.length>0&&<div role="alert" className="q36-filter-errors">{validation.issues.map((issue,index)=><p key={`${issue.path}-${index}`}>{issue.path==='root'?'筛选':`条件 ${issue.path.slice(5).split('.').map(i=>Number(i)+1).join('.')}`}：{issue.message}</p>)}</div>}
          <div className="q36-filter-footer"><button type="button" onClick={()=>setDraft(initialTree(undefined))}>重置条件</button><span>{leafCount} / 20</span>
            <button type="button" className="q36-filter-apply" disabled={validation.issues.length>0} onClick={()=>{if(validation.issues.length)return;onApply(validation.filter);changeOpen(false);}}>应用筛选</button>
          </div>
        </div>;
  const sharedComplete=()=>{
    // The class only suppresses root crossfading while the panel shell moves.
    window.setTimeout(()=>document.documentElement.classList.remove('q36-filter-transition-active'),330);
  };
  return <Popover.Root open={open} onOpenChange={(next,details)=>{if(!next)details.preventUnmountOnClose();changeOpen(next);}} actionsRef={actions}>
    <Popover.Trigger ref={trigger} className="q36-filter-trigger" aria-label={value?'自定义筛选，已应用':'自定义筛选'}>
      <span className="q36-filter-trigger-content"><Funnel size={16} weight={value?'fill':'regular'}/><span>自定义筛选</span>{value&&<span className="q36-filter-count">{countLeaves(initialTree(value))}</span>}</span>
      {sharedMotion&&!open&&<ViewTransition name={sharedName} default="none" share="q36-filter-collapse" onShare={sharedComplete}>
        <span className="q36-filter-trigger-frame" aria-hidden="true"/>
      </ViewTransition>}
    </Popover.Trigger>
    <Popover.Portal keepMounted><Popover.Positioner side="bottom" align="start" sideOffset={8} collisionPadding={12} className="q36-filter-positioner">
      <Popover.Popup ref={setPopup} id={panelId} className="q36-filter-popup" aria-label="自定义筛选" aria-hidden={!open} inert={!open} initialFocus={firstControl} finalFocus={trigger}>
        {sharedMotion ? open&&<ViewTransition name={sharedName} default="none" share="q36-filter-expand" onShare={sharedComplete}>
          <div className="q36-filter-shell" aria-hidden="true"/>
        </ViewTransition> : <div className="q36-filter-shell" aria-hidden="true"/>}
        {sharedMotion ? open&&<ViewTransition default="none" enter="q36-filter-content-in" exit="q36-filter-content-out">{content}</ViewTransition> : content}
      </Popover.Popup>
    </Popover.Positioner></Popover.Portal>
  </Popover.Root>;
}
