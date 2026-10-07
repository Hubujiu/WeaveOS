import {startTransition,useEffect,useId,useLayoutEffect,useRef,useState,ViewTransition} from 'react';
import {Popover} from '@base-ui/react/popover';
import {Funnel,X,Plus,ArrowLeft,ArrowRight} from '@phosphor-icons/react';
import {workspaceApi,WorkspaceError} from './workspace-api';
import {Modal} from './Modal';
import {memberFilterFields,eventFilterFields,filterOperators,MAX_FILTER_LEAVES,type QueryView,type ResourceFilterField,type ResourceFilterGroup} from './QueryFilterState';
import {blockFilter,presetBlocks,newPresetRow,validatePreset,validateResourcePreset,editablePresetFilter,memberBusinessColumns,eventBusinessColumns,type TablePreset,type AppliedPreset,type PresetBlock,type PresetRow,type PresetOptions,type BusinessColumn} from './TablePresetState';
import './query-filter.css';
import './table-presets.css';

export type ResourcePreset = {id:string;name:string;version:number;invalid?:boolean;reason?:string;filter?:ResourceFilterGroup|null;hiddenColumnIds?:string[]};
export type ResourcePresetRepository = {list:()=>Promise<{items:ResourcePreset[]}>;read:(id:string)=>Promise<ResourcePreset>;save:(input:{name:string;filter:ResourceFilterGroup|null;hiddenColumnIds:string[]},existing?:{id:string;version:number})=>Promise<ResourcePreset>;remove:(id:string,version:number)=>Promise<void>};
export type ResourceManagerConfig = {scopeKey:string;fields:readonly ResourceFilterField[];columns:readonly BusinessColumn[];repository:ResourcePresetRepository;refreshDescriptors:()=>Promise<{fields:readonly ResourceFilterField[];columns:readonly BusinessColumn[]}>;onApply:(preset:ResourcePreset|null)=>Promise<boolean>};
type CommonProps={hiddenColumnIds:readonly string[];onDirty:(dirty:boolean)=>void;onUnauthorized:()=>void};
type Props=(CommonProps & {view:QueryView;active:AppliedPreset|null;options?:PresetOptions;loadOptions:()=>Promise<PresetOptions>;onApply:(preset:TablePreset|null)=>Promise<boolean>;resource?:never})|(CommonProps & {view:'resource';resource:ResourceManagerConfig;active:ResourcePreset|null;options?:never;loadOptions?:never;onApply?:never});
type ManagedPreset=TablePreset|ResourcePreset;
type Edit={preset:ManagedPreset|null;name:string;blocks:PresetBlock[];hidden:string[];initial:string};
const contentKey=(e:Pick<Edit,'name'|'blocks'|'hidden'>,key:'field'|'fieldId')=>JSON.stringify([e.name,blockFilter(e.blocks,key),e.hidden]);
export function TablePresetManager(props:Props){
 const {view,active,hiddenColumnIds,onDirty,onUnauthorized}=props,resource=view==='resource'?props.resource:null;
 const options=view==='resource'?{}:props.options??{};
 const loadOptions=view==='resource'?async()=>({}):props.loadOptions;
 const onApply=view==='resource'?async (_preset:TablePreset|null)=>false:props.onApply;
 const filterKey=view==='resource'?'fieldId':'field';
 const [open,setOpen]=useState(false),[expanded,setExpanded]=useState(false),[popup,setPopup]=useState<HTMLDivElement|null>(null);
 const [items,setItems]=useState<ManagedPreset[]>([]),[edit,setEdit]=useState<Edit|null>(null),[error,setError]=useState(''),[busy,setBusy]=useState(false),[listLoading,setListLoading]=useState(false),[conflict,setConflict]=useState(false);
 const [confirmation,setConfirmation]=useState<{kind:'cancel'|'delete'|'leave'|'reload';preset?:ManagedPreset;returnToManager?:boolean}|null>(null);
 const [reducedMotion,setReducedMotion]=useState(()=>matchMedia('(prefers-reduced-motion: reduce)').matches);
 const trigger=useRef<HTMLButtonElement>(null),firstControl=useRef<HTMLButtonElement>(null),nameControl=useRef<HTMLInputElement>(null),actions=useRef<Popover.Root.Actions|null>(null);
 const intent=useRef(false),everOpened=useRef(false),generation=useRef(0),animation=useRef<Animation[]>([]),sharedGeneration=useRef(0),sharedFrame=useRef(0),sharedTimer=useRef(0),requests=useRef(0);
 const outsideClose=useRef<(()=>void)|null>(null);
 const focusLifecycle=useRef(0),closeFocus=useRef<{token:number;popup:HTMLDivElement|null}|null>(null);
 const sharedVersion=sharedGeneration.current,focusVersion=focusLifecycle.current,panelId=useId(),sharedName=`q36-preset-shell-${panelId.replace(/[^a-zA-Z0-9_-]/g,'-')}`;
 const sharedMotion=!reducedMotion&&typeof document.startViewTransition==='function'&&navigator.vendor!=='Apple Computer, Inc.';
 const columns=view==='resource'?resource!.columns:view==='members'?memberBusinessColumns:eventBusinessColumns;
 const fields=view==='resource'?resource!.fields.map(f=>({key:f.id,label:f.label??'字段',kind:f.kind,choices:f.choices,operators:f.operators})):view==='members'?memberFilterFields:eventFilterFields;
 const [hiddenSearch,setHiddenSearch]=useState(''),[shownSearch,setShownSearch]=useState('');
 const applyCurrent=useRef(onApply);applyCurrent.current=onApply;
 const dirty=!!edit&&contentKey(edit,filterKey)!==edit.initial;
 useLayoutEffect(()=>{onDirty(dirty);return()=>onDirty(false);},[dirty,onDirty]);
 useEffect(()=>{if(edit)nameControl.current?.focus({preventScroll:true});},[edit?.preset?.id,!!edit]);
 function failed(e:unknown){if(e instanceof WorkspaceError&&e.status===401)onUnauthorized();setError(e instanceof Error?e.message:'服务暂时不可用，请稍后重试');if(e instanceof WorkspaceError&&e.code==='PERSONNEL_PRESET_CONFLICT')setConflict(true);}
 async function load(){const token=++requests.current;setListLoading(true);setError('');try{const result=resource?await resource.repository.list():await workspaceApi<{items:TablePreset[]}>('personnel/table-presets?view='+view);if(token===requests.current)setItems(result.items);}catch(e){if(token===requests.current)failed(e);}finally{if(token===requests.current)setListLoading(false);}}
 useEffect(()=>{setItems([]);setEdit(null);setConfirmation(null);if(open)void load();return()=>{requests.current++;};},[open,view,resource?.scopeKey]);
 function invalidateCloseFocus(){focusLifecycle.current++;closeFocus.current=null;}
 function restoreCloseFocus(token:number,keyboard=false){
  const pending=closeFocus.current,button=trigger.current;
  if(!pending||pending.token!==token||token!==focusLifecycle.current||intent.current||!button?.isConnected)return;
  const active=document.activeElement;
  if(active&&active!==document.body&&active!==document.documentElement&&active!==button&&!pending.popup?.contains(active)){invalidateCloseFocus();return;}
  // One close owns one restoration. Body focus caused by hiding/removing the
  // popup still needs restoring, unless newer deliberate input revoked it.
  closeFocus.current=null;
  if(active!==button)button.focus({preventScroll:true,...(keyboard?{focusVisible:true}:{})});
 }
 // Returning a target lets Base UI queue another focus transfer. Perform the
 // restoration under the shared authority here, then suppress that extra move.
 const finalFocus:Popover.Popup.Props['finalFocus']=closeType=>{restoreCloseFocus(focusLifecycle.current,closeType==='keyboard');return false;};
 useLayoutEffect(()=>{
  function newerInteraction(event:Event){
   const pending=closeFocus.current;if(!pending)return;
   if(event.type==='focusin'){
    const target=event.target as Node|null;
    if(target===document.body||target===document.documentElement||target===trigger.current||pending.popup?.contains(target))return;
   }else if(event.type==='keydown'&&['Escape','Shift','Control','Alt','Meta'].includes((event as KeyboardEvent).key))return;
   invalidateCloseFocus();
  }
  for(const type of ['focusin','pointerdown','keydown'])document.addEventListener(type,newerInteraction,true);
  return()=>{for(const type of ['focusin','pointerdown','keydown'])document.removeEventListener(type,newerInteraction,true);};
 },[]);
 function finishShared(version:number,focusToken:number){
  if(version!==sharedGeneration.current)return;clearTimeout(sharedTimer.current);document.documentElement.classList.remove('q36-filter-transition-active','q36-preset-transition-active');
  if(!intent.current){setExpanded(false);setOpen(false);restoreCloseFocus(focusToken);}
 }
 function sharedComplete(version:number,focusToken:number){
  if(version!==sharedGeneration.current)return;requestAnimationFrame(()=>{if(version!==sharedGeneration.current)return;
   if(intent.current&&(document.activeElement===popup||!popup?.contains(document.activeElement)))firstControl.current?.focus({preventScroll:true});
   const motions=document.getAnimations().filter(a=>a.effect instanceof KeyframeEffect&&a.effect.pseudoElement?.startsWith('::view-transition'));
   Promise.allSettled(motions.map(a=>a.finished)).then(()=>finishShared(version,focusToken));
  });
 }
 function changeOpen(next:boolean,restoreFocus=true){
  outsideClose.current?.();
  if(next||next!==intent.current){invalidateCloseFocus();if(!next&&restoreFocus)closeFocus.current={token:focusLifecycle.current,popup};}else if(!restoreFocus)invalidateCloseFocus();
  intent.current=next;if(next)everOpened.current=true;const focusToken=focusLifecycle.current;
  if(sharedMotion){const version=++sharedGeneration.current;cancelAnimationFrame(sharedFrame.current);clearTimeout(sharedTimer.current);
   const exchange=()=>{if(version!==sharedGeneration.current||next!==intent.current)return;document.documentElement.style.setProperty('--q36-filter-shell-duration',next?'300ms':'220ms');document.documentElement.classList.add('q36-filter-transition-active','q36-preset-transition-active');sharedTimer.current=window.setTimeout(()=>finishShared(version,focusToken),600);startTransition(()=>setExpanded(previous=>version===sharedGeneration.current&&next===intent.current?next:previous));};
   if(next){setOpen(true);sharedFrame.current=requestAnimationFrame(()=>{sharedFrame.current=requestAnimationFrame(exchange);});}
   else if(expanded)exchange();else{setExpanded(false);setOpen(false);finishShared(version,focusToken);}
  }else{setExpanded(next);setOpen(next);}
 }
 function requestClose(restore=true){if(busy||confirmation)return;if(dirty){setConfirmation({kind:'leave'});return;}setEdit(null);setError('');changeOpen(false,restore);}
 function deferOutsideClose(event:Event){
  outsideClose.current?.();invalidateCloseFocus();
  let active=true,timer=0;
  // Starting a React ViewTransition on mousedown can suppress React's later
  // click dispatch during its async commit. Let the real outside gesture finish;
  // never redispatch a synthetic click or change the user's target.
  const cleanup=()=>{active=false;clearTimeout(timer);document.removeEventListener('click',queue);document.removeEventListener('pointerup',queue);document.removeEventListener('pointercancel',queue);window.removeEventListener('blur',queue);if(outsideClose.current===cleanup)outsideClose.current=null;};
  const complete=()=>{if(!active)return;cleanup();if(intent.current)requestClose(false);};
  const queue=()=>{clearTimeout(timer);timer=window.setTimeout(complete,0);};
  outsideClose.current=cleanup;
  document.addEventListener('click',queue);document.addEventListener('pointerup',queue);document.addEventListener('pointercancel',queue);window.addEventListener('blur',queue);
  if(!['mousedown','pointerdown','touchstart'].includes(event.type))queue();
 }
 useLayoutEffect(()=>{
  function beginOutsideGesture(event:PointerEvent){
   if(!sharedMotion||dirty||busy||confirmation||!intent.current)return;
   const target=event.target;
   if(target instanceof Node&&!popup?.contains(target)&&!trigger.current?.contains(target))deferOutsideClose(event);
  }
  // Native capture precedes focus-out, which can arrive before outside-press.
  document.addEventListener('pointerdown',beginOutsideGesture,true);
  return()=>document.removeEventListener('pointerdown',beginOutsideGesture,true);
 },[sharedMotion,dirty,busy,confirmation,popup]);
 useLayoutEffect(()=>{const media=matchMedia('(prefers-reduced-motion: reduce)');const sync=()=>setReducedMotion(media.matches);media.addEventListener('change',sync);return()=>media.removeEventListener('change',sync);},[]);
 useLayoutEffect(()=>{
  if(!popup||sharedMotion||!everOpened.current)return;const version=++generation.current,focusToken=focusLifecycle.current;
  const frame=requestAnimationFrame(()=>{if(version!==generation.current)return;
   const shell=popup.querySelector<HTMLElement>('.q36-filter-shell'),content=popup.querySelector<HTMLElement>('.q36-filter-content'),previous=animation.current;
   const transform=shell?getComputedStyle(shell).transform:'none',opacity=content?getComputedStyle(content).opacity:'0';previous.forEach(a=>a.cancel());animation.current=[];
   const complete=()=>{if(version===generation.current&&!intent.current){actions.current?.unmount();restoreCloseFocus(focusToken);}};
   if(reducedMotion||!shell?.animate){complete();return;}const from=trigger.current?.getBoundingClientRect(),to=popup.getBoundingClientRect();if(!from||!to.width||!to.height){complete();return;}
   const small=`translate(${from.left-to.left}px,${from.top-to.top}px) scale(${from.width/to.width},${from.height/to.height})`;
   const motion=shell.animate([{transform:previous.length?transform:open?small:'none'},{transform:open?'none':small}],{duration:open?300:220,easing:'cubic-bezier(.2,.8,.2,1)',fill:'both'});animation.current.push(motion);
   if(content)animation.current.push(content.animate([{opacity:previous.length?opacity:open?0:1},{opacity:open?1:0}],{duration:open?160:80,delay:open?100:0,fill:'both',easing:'ease-out'}));motion.finished.then(complete,()=>{});
  });return()=>{generation.current++;cancelAnimationFrame(frame);};
 },[open,popup,sharedMotion,reducedMotion]);
 useLayoutEffect(()=>()=>{outsideClose.current?.();invalidateCloseFocus();requests.current++;generation.current++;sharedGeneration.current++;animation.current.forEach(a=>a.cancel());cancelAnimationFrame(sharedFrame.current);clearTimeout(sharedTimer.current);document.documentElement.classList.remove('q36-filter-transition-active','q36-preset-transition-active');},[]);
 function begin(preset:ManagedPreset|null){const invalid=resource&&preset&&'invalid'in preset&&preset.invalid;const next={preset,name:preset?.name??'',blocks:presetBlocks(invalid?null:preset?.filter,filterKey),hidden:[...(invalid?hiddenColumnIds:(preset?.hiddenColumnIds??hiddenColumnIds))],initial:''};next.initial=contentKey(next,filterKey);setEdit(next);setConflict(false);setError('');setHiddenSearch('');setShownSearch('');}
 async function readEdit(item:ManagedPreset){if(busy)return;setBusy(true);setError('');try{if(resource&&'invalid'in item&&item.invalid){begin(item);return;}const latest=resource?await resource.repository.read(item.id):await workspaceApi<TablePreset>('personnel/table-presets/'+item.id);if(resource&&'invalid'in latest&&latest.invalid){begin(latest);return;}if(!editablePresetFilter(latest.filter))throw new Error('方案包含当前编辑器无法表达的筛选树，已保留原条件；不能静默转换或保存。');begin(latest);}catch(e){failed(e);}finally{setBusy(false);}}
 function patchRow(blockId:string,rowId:string,patch:Partial<PresetRow>){setEdit(e=>e?{...e,blocks:e.blocks.map(b=>b.id===blockId?{...b,rows:b.rows.map(r=>r.id===rowId?{...r,...patch}:r)}:b)}:e);}
 function newRow():PresetRow{return resource?{id:crypto.randomUUID(),field:resource.fields[0]?.id??'',operator:'eq',value:''}:newPresetRow(view as QueryView);}
 function addBlock(){setEdit(e=>e?{...e,blocks:[...e.blocks,{id:crypto.randomUUID(),rows:[newRow()]}]}:e);}
 async function save(){if(!edit||busy)return;setError('');const filter=blockFilter(edit.blocks,filterKey);const v=resource?validateResourcePreset(edit.name,filter,edit.hidden,resource.fields,resource.columns):validatePreset(view as QueryView,edit.name,filter,edit.hidden,options);if(v.issues.length){setError(v.issues.join('\n'));return;}setBusy(true);try{
  const stored=resource?await resource.repository.save({name:v.name,filter:v.filter as ResourceFilterGroup|null,hiddenColumnIds:edit.hidden},edit.preset?{id:edit.preset.id,version:edit.preset.version}:undefined):await workspaceApi<TablePreset>('personnel/table-presets'+(edit.preset?'/'+edit.preset.id:''),edit.preset?'PUT':'POST',edit.preset?{name:v.name,filter:v.filter,hiddenColumnIds:edit.hidden,schemaVersion:1,version:edit.preset.version}:{name:v.name,filter:v.filter,hiddenColumnIds:edit.hidden,schemaVersion:1,view});
  setItems(old=>[stored,...old.filter(p=>p.id!==stored.id)]);setEdit(null);setConflict(false);
 }catch(e){failed(e);}finally{setBusy(false);}}
 async function apply(item:ManagedPreset){if(busy)return;setBusy(true);setError('');try{
  if(resource){if('invalid'in item&&item.invalid)throw new Error(item.reason||'方案已失效，请明确重新配置');const latest=await resource.repository.read(item.id);if(latest.invalid)throw new Error(latest.reason||'方案已失效，请明确重新配置');const current=await resource.refreshDescriptors();const v=validateResourcePreset(latest.name,latest.filter,latest.hiddenColumnIds??[],current.fields,current.columns);if(v.issues.length)throw new Error(v.issues.join('\n'));if(await resource.onApply({...latest,filter:v.filter,hiddenColumnIds:[...latest.hiddenColumnIds??[]]}))setItems(old=>old.map(p=>p.id===latest.id?latest:p));return;}
  const latest=await workspaceApi<TablePreset>('personnel/table-presets/'+item.id),freshOptions=await loadOptions();
  const v=validatePreset(view as QueryView,latest.name,latest.filter,latest.hiddenColumnIds,freshOptions);if(latest.view!==view||latest.schemaVersion!==1)throw new Error('方案所属表格或结构已失效，请重新加载后编辑');if(v.issues.length)throw new Error(v.issues.join('\n'));
  if(await applyCurrent.current(latest))setItems(old=>old.map(p=>p.id===latest.id?latest:p));
 }catch(e){failed(e);}finally{setBusy(false);}}
 async function confirm(){if(!confirmation||busy)return;const c=confirmation;if(c.kind==='leave'){setEdit(null);setConfirmation(null);setError('');if(!c.returnToManager)changeOpen(false);return;}if(c.kind==='reload'){setConfirmation(null);if(edit?.preset)await readEdit(edit.preset);return;}
  setBusy(true);setError('');let removed=false;try{
   if(c.kind==='cancel'){if(await (resource?resource.onApply(null):applyCurrent.current(null)))setConfirmation(null);return;}
   if(!c.preset)return;
   const remove=async()=>{if(resource)await resource.repository.remove(c.preset!.id,c.preset!.version);else await workspaceApi('personnel/table-presets/'+c.preset!.id+'?version='+c.preset!.version,'DELETE');};
   await remove();removed=true;setItems(old=>old.filter(p=>p.id!==c.preset!.id));setConfirmation(null);
   if(active?.id===c.preset.id&&!(await (resource?resource.onApply(null):applyCurrent.current(null))))setError('方案已删除；当前查询已被更新的操作取代，应用快照仍保留。请显式刷新查询后取消应用。');
  }catch(e){failed(e);if(removed)setError('方案已删除，当前应用尚未解除；'+(e instanceof Error?e.message:'查询失败，请显式刷新后取消应用'));}finally{setBusy(false);}
 }
 const leaves=edit?.blocks.reduce((n,b)=>n+b.rows.length,0)??0;
 function row(b:PresetBlock,r:PresetRow,bi:number,ri:number){
  const field=fields.find(f=>f.key===r.field),resourceField=resource?.fields.find(f=>f.id===r.field),path=`${bi+1}.${ri+1}`,label=`条件 ${path}`,choices=r.field==='departmentIds'?options.departmentIds:r.field==='identityIds'?options.identityIds:field?.choices;
  const calendar=!resource&&!Array.isArray(r.value)&&typeof r.value==='object'&&r.value!==null?r.value as {date:string;timeZone:string}:null,mode=r.value===null?'null':calendar?'date':'value';
  const operators=resourceField?filterOperators.filter(o=>resourceField.operators.includes(o.value)):(field?.kind==='time'?filterOperators:filterOperators.slice(0,2));
  return <div key={r.id} className="preset-condition">
   <select aria-label={label+' 字段'} value={r.field} onChange={e=>patchRow(b.id,r.id,{field:e.target.value,operator:'eq',value:''})}><option value="">请选择字段</option>{!field&&r.field&&<option value={r.field}>{resource?'字段已不可用':'已失效字段：'+r.field}</option>}{fields.map(f=><option key={f.key} value={f.key}>{f.label}{edit?.hidden.includes(f.key==='departmentIds'?'departments':f.key==='identityIds'?'identities':f.key)?'（已隐藏）':''}</option>)}</select>
   <select aria-label={label+' 比较'} value={r.operator} onChange={e=>patchRow(b.id,r.id,{operator:e.target.value,...(!['eq','neq'].includes(e.target.value)&&r.value===null?{value:''}:{})})}>{operators.map(o=><option key={o.value} value={o.value}>{o.label}</option>)}</select>
   {field?.kind!=='relation'&&field?.kind!=='member'&&field?.kind!=='department'&&<select className="preset-value-mode" aria-label={label+' 值类型'} value={mode} onChange={e=>patchRow(b.id,r.id,{value:e.target.value==='null'?null:e.target.value==='date'?{date:'',timeZone:Intl.DateTimeFormat().resolvedOptions().timeZone}:''})}><option value="value">{field?.kind==='time'||field?.kind==='datetime'?'绝对时刻':'值'}</option>{field?.kind==='time'&&<option value="date">日期及时区</option>}{['eq','neq'].includes(r.operator)&&<option value="null">空值</option>}</select>}
   {mode==='null'?<span className="preset-null">空值（NULL）</span>:calendar?<div className="preset-date"><input type="date" aria-label={label+' 日期'} value={calendar.date} onChange={e=>patchRow(b.id,r.id,{value:{...calendar,date:e.target.value}})}/><input aria-label={label+' 时区'} value={calendar.timeZone} onChange={e=>patchRow(b.id,r.id,{value:{...calendar,timeZone:e.target.value}})}/></div>:resourceField?.kind==='multi_select'?<select multiple aria-label={label+' 值'} value={Array.isArray(r.value)?r.value:[]} onChange={e=>patchRow(b.id,r.id,{value:Array.from(e.target.selectedOptions,o=>o.value)})}>{choices?.map(c=><option key={c.value} value={c.value}>{c.label}</option>)}</select>:resourceField?.kind==='date'?<input type="date" aria-label={label+' 值'} value={typeof r.value==='string'?r.value:''} onChange={e=>patchRow(b.id,r.id,{value:e.target.value})}/>:resourceField?.kind==='member'||resourceField?.kind==='department'?<span role="status">引用候选选择器尚未接入</span>:field?.kind==='enum'||field?.kind==='single_select'||field?.kind==='relation'||field?.kind==='boolean'?<select aria-label={label+' 值'} value={typeof r.value==='boolean'?String(r.value):String(r.value??'')} onChange={e=>patchRow(b.id,r.id,{value:field?.kind==='boolean'?e.target.value===''?'':e.target.value==='true':e.target.value})}><option value="">请选择</option>{field.kind==='boolean'?<><option value="true">是</option><option value="false">否</option></>:choices?.map(c=><option key={c.value} value={c.value}>{c.label}</option>)}{!resource&&field.kind==='relation'&&typeof r.value==='string'&&r.value&&!choices?.some(c=>c.value===r.value)&&<option value={r.value}>已失效引用：{r.value}</option>}</select>:<input aria-label={label+' 值'} inputMode={resourceField?.kind==='money'||resourceField?.kind==='number'?'decimal':undefined} value={typeof r.value==='string'?r.value:''} placeholder={field?.kind==='time'||field?.kind==='datetime'?'2026-10-02T10:00:00+08:00':'请输入值'} onChange={e=>patchRow(b.id,r.id,{value:e.target.value})}/>}
   <button type="button" className="preset-icon" aria-label={'删除'+label} onClick={()=>setEdit(e=>e?{...e,blocks:e.blocks.flatMap(block=>block.id===b.id?block.rows.length===1?[]:[{...block,rows:block.rows.filter(row=>row.id!==r.id)}]:[block])}:e)}><X size={14}/></button>
  </div>;
 }
 function visibility(hidden:boolean){const search=hidden?hiddenSearch:shownSearch;const list=columns.filter(c=>edit?.hidden.includes(c.id)===hidden&&c.name.toLocaleLowerCase().includes(search.toLocaleLowerCase()));return <section className="preset-column-list" aria-label={hidden?'隐藏字段':'显示字段'}><h4>{hidden?'隐藏字段':'显示字段'}</h4><input aria-label={hidden?'隐藏字段搜索':'显示字段搜索'} placeholder="输入关键词进行搜索" value={search} onChange={e=>(hidden?setHiddenSearch:setShownSearch)(e.target.value)}/><div className="preset-column-items">{list.length?list.map(c=><div key={c.id} className="preset-column-row"><span>{c.name}</span><button type="button" className="preset-icon" disabled={!hidden&&edit?.hidden.length===columns.length-1} aria-label={(hidden?'显示':'隐藏')+c.name} onClick={()=>setEdit(e=>e?{...e,hidden:hidden?e.hidden.filter(id=>id!==c.id):[...e.hidden,c.id]}:e)}>{hidden?<ArrowRight size={16}/>:<ArrowLeft size={16}/>}</button></div>):<p className="preset-empty">暂无字段</p>}</div></section>;}
 const title=edit?edit.preset?'编辑自定义筛选':'新增自定义筛选':'管理自定义筛选';
 const content=<div className="q36-filter-content preset-content" style={{visibility:sharedMotion&&!expanded?'hidden':undefined}}>
  <header className="preset-heading"><Popover.Title>{title}</Popover.Title><button type="button" className="preset-icon preset-close" aria-label="关闭筛选管理" disabled={busy} onClick={()=>requestClose()}><X size={18}/></button></header>
  <div className="preset-body">
   {error&&<div role="alert" className="preset-error">{error}</div>}
   {edit?<><label className="preset-name">自定义筛选名称 <span aria-hidden="true">*</span><input ref={nameControl} aria-label="自定义筛选名称" required placeholder="请输入自定义筛选名称" value={edit.name} onChange={e=>setEdit(old=>old?{...old,name:e.target.value}:old)}/></label><h3>配置筛选条件</h3>
    {edit.blocks.map((b,bi)=><div key={b.id}>{bi>0&&<div className="preset-or"><span>或</span></div>}<fieldset className="preset-block" aria-label={`且条件组 ${bi+1}`}><legend>且条件</legend>{b.rows.map((r,ri)=>row(b,r,bi,ri))}<button type="button" className="preset-add" aria-label={`组 ${bi+1} 且条件`} disabled={leaves>=MAX_FILTER_LEAVES} onClick={()=>setEdit(e=>e?{...e,blocks:e.blocks.map(block=>block.id===b.id?{...block,rows:[...block.rows,newRow()]}:block)}:e)}><Plus size={14}/>且条件</button></fieldset></div>)}
    <button type="button" className="preset-add" aria-label="或条件" disabled={leaves>=MAX_FILTER_LEAVES} onClick={addBlock}><Plus size={14}/>或条件</button><h3>设置显隐字段</h3><div className="preset-visibility">{visibility(true)}{visibility(false)}</div><p className="preset-hint">隐藏字段仍参与筛选与排序。至少保留一个业务字段；选择、序号和操作列始终显示。</p>
    {conflict&&edit.preset&&<button type="button" onClick={()=>setConfirmation({kind:'reload'})}>重新加载最新方案</button>}
   </>:<><button ref={firstControl} type="button" className="preset-add preset-new" aria-label="新增筛选" disabled={busy||listLoading||items.length>=20} onClick={()=>begin(null)}><Plus size={16}/>新增筛选</button>
    {listLoading?<p role="status" className="preset-empty">正在加载筛选方案…</p>:items.length?<ul className="preset-list">{items.map(item=><li key={item.id}><div className="preset-info"><strong>{item.name}</strong>{resource&&'invalid'in item&&item.invalid&&<span className="preset-error">{item.reason||'方案已失效'}</span>}{active?.id===item.id&&<span className="preset-active">{active.version===item.version?'已应用':'已修改，待应用'}</span>}</div><div className="preset-actions"><button type="button" disabled={busy||!!(resource&&'invalid'in item&&item.invalid)} aria-label={'应用'+item.name} onClick={()=>void apply(item)}>应用</button><button type="button" disabled={busy} aria-label={'编辑'+item.name} onClick={()=>void readEdit(item)}>编辑</button><button type="button" disabled={busy} aria-label={'删除'+item.name} onClick={()=>{setError('');setConfirmation({kind:'delete',preset:item});}}>删除</button></div></li>)}</ul>:<p className="preset-empty">暂无自定义筛选</p>}
    {active&&<button type="button" className="preset-cancel-application" disabled={busy} onClick={()=>setConfirmation({kind:'cancel'})}>取消应用</button>}{error&&<button type="button" disabled={busy} onClick={()=>void load()}>重新加载方案列表</button>}
   </>}
  </div>
  {edit&&<footer className="preset-footer"><span>{leaves} / 20 条件</span><button type="button" disabled={busy} onClick={()=>{if(dirty)setConfirmation({kind:'leave',returnToManager:true});else{setEdit(null);setError('');}}}>取消</button><button type="button" className="preset-primary" disabled={busy} onClick={()=>void save()}>{busy?'保存中…':'确定'}</button></footer>}
 </div>;
 return <>
  <Popover.Root open={open} onOpenChange={(next,details)=>{const requested=details.reason==='trigger-press'?!intent.current:next;if(!requested){details.preventUnmountOnClose();if(outsideClose.current&&(details.reason==='outside-press'||details.reason==='focus-out')){details.cancel();return;}if(details.reason==='outside-press'&&sharedMotion&&!dirty&&!busy&&!confirmation){details.cancel();deferOutsideClose(details.event);}else requestClose(details.reason!=='outside-press');}else changeOpen(true);}} actionsRef={actions}>
   <Popover.Trigger ref={trigger} className="q36-filter-trigger" aria-label={active?'自定义筛选，已应用':'自定义筛选'}><ViewTransition default="none" update={expanded?'q36-filter-trigger-out':'q36-filter-trigger-in'}><span className="q36-filter-trigger-content" style={{visibility:sharedMotion&&expanded?'hidden':undefined}}><Funnel size={16} weight={active?'fill':'regular'}/><span>自定义筛选</span>{active&&<span className="q36-filter-count">1</span>}</span></ViewTransition>{sharedMotion&&!expanded&&<ViewTransition name={sharedName} default="none" share="q36-filter-shell-motion" enter="q36-filter-shell-motion" exit="q36-filter-shell-motion" onShare={()=>sharedComplete(sharedVersion,focusVersion)} onEnter={()=>sharedComplete(sharedVersion,focusVersion)} onExit={()=>sharedComplete(sharedVersion,focusVersion)}><span className="q36-filter-trigger-frame" aria-hidden="true"/></ViewTransition>}</Popover.Trigger>
   <Popover.Portal keepMounted><Popover.Positioner className="preset-positioner" side="bottom" align="start" collisionPadding={12}><Popover.Popup ref={setPopup} id={panelId} className={'preset-popup '+(edit?'preset-editor':'preset-manager')} aria-label={title} aria-hidden={sharedMotion?!expanded:!open} inert={sharedMotion?!expanded:!open} initialFocus={firstControl} finalFocus={finalFocus}>
    {sharedMotion?expanded&&<ViewTransition name={sharedName} default="none" share="q36-filter-shell-motion" enter="q36-filter-shell-motion" exit="q36-filter-shell-motion" onShare={()=>sharedComplete(sharedVersion,focusVersion)} onEnter={()=>sharedComplete(sharedVersion,focusVersion)} onExit={()=>sharedComplete(sharedVersion,focusVersion)}><div className="q36-filter-shell" aria-hidden="true"/></ViewTransition>:<div className="q36-filter-shell" aria-hidden="true"/>}
    {sharedMotion?<ViewTransition default="none" update={expanded?'q36-filter-content-in':'q36-filter-content-out'}>{content}</ViewTransition>:content}
   </Popover.Popup></Popover.Positioner></Popover.Portal>
  </Popover.Root>
  {confirmation&&<Modal title={confirmation.kind==='cancel'?'取消当前筛选':confirmation.kind==='delete'?'删除筛选方案':confirmation.kind==='reload'?'重新加载最新方案':'放弃未保存的筛选'} busy={busy} onClose={()=>setConfirmation(null)}><p>{confirmation.kind==='delete'?`确认删除“${confirmation.preset?.name}”？${active?.id===confirmation.preset?.id?'同时取消当前应用，并恢复首次应用前的显隐字段。':''}`:confirmation.kind==='cancel'?'取消当前应用，并恢复首次应用前的显隐字段。':confirmation.kind==='reload'?'将丢弃当前输入，并读取最新版本。':'当前修改尚未保存，确认放弃更改？'}</p>{error&&<p role="alert">{error}</p>}<div className="preset-confirm-actions"><button type="button" disabled={busy} onClick={()=>setConfirmation(null)}>返回</button><button type="button" disabled={busy} className="preset-primary" onClick={()=>void confirm()}>{confirmation.kind==='delete'?'确认删除':confirmation.kind==='cancel'?'确认取消':confirmation.kind==='reload'?'确认重新加载':'放弃更改'}</button></div></Modal>}
 </>;
}
