import { useEffect, useRef, useState } from 'react';
import type {
  DecimalConfig, Field, FieldInput, FieldKind, LayoutNode, LayoutNodeInput, Option, SystemFieldCode, UUID,
} from './contracts';

export type FieldValue = string | boolean | UUID[] | null;
export type ReferenceOption = { id: UUID; label: string };
/** The safe subset shared by owner definitions and V017 RuntimeField projections. */
export type DisplayField = {
  id: UUID; name: string; kind: FieldKind; required: boolean;
  presentation: {helpText: string | null};
  input?: {options?: Option[];referenceKind?:'member'|'department';decimal?:DecimalConfig;
    timePrecision?:'minute'|'second'|'millisecond'};
  config?: unknown;
};
export type ReferenceDisplay = {id:UUID;label:string;deleted:boolean};
export type ReferenceCandidate = {id:UUID;label:string;status:'active'|'disabled'|'deleted';parentId?:UUID|null};
export type ReferenceCandidatePage = {items:ReferenceCandidate[];nextPageToken:string|null};
export type LoadReferenceCandidates = (
  request:{q:string;pageSize:number;pageToken:string|null},signal:AbortSignal,
) => Promise<ReferenceCandidatePage>;
export type FieldRendererProps = {
  field: DisplayField;
  value: FieldValue;
  onChange?: (value: FieldValue) => void;
  readOnly?: boolean;
  referenceOptions?: ReferenceOption[];
  referenceDisplay?: ReferenceDisplay|null;
  loadReferenceCandidates?: LoadReferenceCandidates;
  referenceScopeKey?: string;
  idPrefix?: string;
};

function ownerConfig(field:DisplayField):Record<string,unknown>{
  return field.config&&typeof field.config==='object'&&!Array.isArray(field.config)
    ?field.config as Record<string,unknown>:{};
}
function maxLength(field:DisplayField){
  const value=ownerConfig(field).maxLength;
  return typeof value==='number'?value:undefined;
}
function options(field:DisplayField):Option[]{
  const value=field.input?.options??ownerConfig(field).options;
  return Array.isArray(value)?value.filter((item):item is Option=>
    !!item&&typeof item==='object'&&typeof item.id==='string'&&typeof item.label==='string'):[];
}

function ReferenceSelector({field,value,onChange,readOnly,referenceOptions,referenceDisplay,loadReferenceCandidates,idPrefix}:
  Pick<FieldRendererProps,'field'|'value'|'onChange'|'readOnly'|'referenceOptions'|'referenceDisplay'|'loadReferenceCandidates'|'idPrefix'>){
  const [open,setOpen]=useState(false);
  const [query,setQuery]=useState('');
  const [items,setItems]=useState<ReferenceCandidate[]>([]);
  const [nextPageToken,setNextPageToken]=useState<string|null>(null);
  const [busy,setBusy]=useState(false);
  const [error,setError]=useState(false);
  const [retryEpoch,setRetryEpoch]=useState(0);
  const [selected,setSelected]=useState<ReferenceOption|null>(null);
  const generation=useRef(0);
  const activeRequest=useRef<AbortController|null>(null);
  const disabled=readOnly||!onChange;
  const inputId=`${idPrefix}-${field.id}`;
  const selectedId=typeof value==='string'?value:null;
  const display=selectedId===referenceDisplay?.id?referenceDisplay:
    selectedId===selected?.id?{...selected,deleted:false}:
    referenceOptions?.find(option=>option.id===selectedId);
  const unavailable=!!selectedId&&!display;
  const referenceName=field.kind==='member'?'成员':'部门';

  useEffect(()=>{
    if(!open||disabled||!loadReferenceCandidates)return;
    const request=new AbortController();
    activeRequest.current?.abort();activeRequest.current=request;
    const current=++generation.current;
    setBusy(true);setError(false);setItems([]);setNextPageToken(null);
    void loadReferenceCandidates({q:query,pageSize:20,pageToken:null},request.signal).then(page=>{
      if(current!==generation.current||request.signal.aborted)return;
      setItems(page.items.filter(item=>item.status==='active'&&
        !(referenceDisplay?.deleted&&item.id===referenceDisplay.id)));
      setNextPageToken(page.nextPageToken);
    }).catch(()=>{if(current===generation.current&&!request.signal.aborted)setError(true);})
      .finally(()=>{if(current===generation.current&&!request.signal.aborted)setBusy(false);});
    return ()=>{request.abort();generation.current++;};
  },[open,disabled,loadReferenceCandidates,query,field.id,referenceDisplay?.id,referenceDisplay?.deleted,retryEpoch]);

  const loadMore=async()=>{
    if(!nextPageToken||busy||!loadReferenceCandidates)return;
    const request=new AbortController();activeRequest.current=request;
    const current=generation.current;
    setBusy(true);setError(false);
    try{
      const page=await loadReferenceCandidates({q:query,pageSize:20,pageToken:nextPageToken},request.signal);
      if(current!==generation.current||request.signal.aborted)return;
      setItems(previous=>{
        const ids=new Set(previous.map(item=>item.id));
        return [...previous,...page.items.filter(item=>item.status==='active'&&!ids.has(item.id)&&
          !(referenceDisplay?.deleted&&item.id===referenceDisplay.id))];
      });
      setNextPageToken(page.nextPageToken);
    }catch{if(current===generation.current&&!request.signal.aborted)setError(true);}
    finally{if(current===generation.current&&!request.signal.aborted)setBusy(false);}
  };
  const currentDisplay=<span id={inputId} className="forms-reference-value">
    {display?.label??(unavailable?'引用信息不可用（需修复）':'未选择')}
    {display&&'deleted'in display&&display.deleted===true&&<span className="forms-reference-deleted">已删除</span>}
  </span>;
  if(!loadReferenceCandidates&&!referenceDisplay&&!readOnly){
    return <select id={inputId} value={selectedId??''} disabled={disabled||!referenceOptions?.length}
      onChange={event=>onChange?.(event.target.value||null)}>
      <option value="">{referenceOptions?.length?'请选择':'来源尚未接入'}</option>
      {referenceOptions?.map(option=><option key={option.id} value={option.id}>{option.label}</option>)}
    </select>;
  }
  return <div className="forms-reference-selector">
    {currentDisplay}
    {!disabled&&loadReferenceCandidates&&<button type="button" aria-expanded={open}
      aria-controls={`${inputId}-candidates`} onClick={()=>setOpen(previous=>!previous)}>选择{referenceName}</button>}
    {open&&!disabled&&loadReferenceCandidates&&<div id={`${inputId}-candidates`} className="forms-reference-candidates">
      <label htmlFor={`${inputId}-search`}>搜索{referenceName}</label>
      <input id={`${inputId}-search`} type="search" value={query}
        onChange={event=>{const next=Array.from(event.target.value.trim()).slice(0,100).join('');
          if(next!==query){setItems([]);setNextPageToken(null);setQuery(next);}}}/>
      {error&&<p role="alert">候选加载失败，请重试</p>}
      {items.map(item=><button type="button" key={item.id} onClick={()=>{
        setSelected(item);onChange?.(item.id);setOpen(false);
      }}>{item.label}</button>)}
      {busy&&<span role="status">正在加载候选</span>}
      {!busy&&!error&&!items.length&&<span>无可选{referenceName}</span>}
      {error&&<button type="button" onClick={()=>setRetryEpoch(previous=>previous+1)}>重试</button>}
      {nextPageToken&&<button type="button" disabled={busy} onClick={()=>void loadMore()}>加载更多</button>}
    </div>}
  </div>;
}

/** Business-field input only. The parent decides when a value is persisted. */
export function FieldRenderer({field,value,onChange,readOnly=false,referenceOptions,referenceDisplay,loadReferenceCandidates,idPrefix='field'}:FieldRendererProps) {
  const disabled=readOnly || !onChange;
  const help=field.presentation.helpText;
  const inputId=`${idPrefix}-${field.id}`;
  let control;
  switch(field.kind) {
    case 'text':
      control=<input id={inputId} type="text" value={typeof value==='string'?value:''}
        maxLength={maxLength(field)} disabled={disabled}
        onChange={event=>onChange?.(event.target.value)}/>;
      break;
    case 'multiline':
      control=<textarea id={inputId} value={typeof value==='string'?value:''}
        maxLength={maxLength(field)} disabled={disabled}
        onChange={event=>onChange?.(event.target.value)}/>;
      break;
    case 'number': case 'money':
      control=<input id={inputId} type="text" inputMode="decimal"
        value={typeof value==='string'?value:''} disabled={disabled}
        placeholder={field.kind==='money'?'0.00':'0'}
        onChange={event=>onChange?.(event.target.value)}/>;
      break;
    case 'date':
      control=<input id={inputId} type="date" value={typeof value==='string'?value:''}
        disabled={disabled} onChange={event=>onChange?.(event.target.value)}/>;
      break;
    case 'datetime':
      control=<input id={inputId} type="text" value={typeof value==='string'?value:''}
        disabled={disabled} placeholder="2026-10-03T09:00:00+08:00"
        onChange={event=>onChange?.(event.target.value)}/>;
      break;
    case 'single_select':
      control=<select id={inputId} value={typeof value==='string'?value:''} disabled={disabled}
        onChange={event=>onChange?.(event.target.value||null)}>
        <option value="">请选择</option>
        {options(field).map(option=><option key={option.id} value={option.id}>{option.label}</option>)}
      </select>;
      break;
    case 'multi_select': {
      const selected=Array.isArray(value)?value:[];
      control=<div className="forms-multi-options" role="group" aria-label={field.name}>
        {options(field).map((option:Option)=><label key={option.id}>
          <input type="checkbox" checked={selected.includes(option.id)} disabled={disabled}
            onChange={event=>onChange?.(event.target.checked
              ? [...selected.filter(id=>id!==option.id),option.id]
              : selected.filter(id=>id!==option.id))}/>{option.label}
        </label>)}
        {!options(field).length&&<span>尚无选项</span>}
      </div>;
      break;
    }
    case 'boolean':
      control=<input id={inputId} type="checkbox" checked={value===true} disabled={disabled}
        onChange={event=>onChange?.(event.target.checked)}/>;
      break;
    case 'member': case 'department':
      control=<ReferenceSelector {...{field,value,onChange,readOnly,referenceOptions,referenceDisplay,loadReferenceCandidates,idPrefix}}/>;
      break;
  }
  return <div className="forms-rendered-field">
    <label htmlFor={inputId} className="forms-field-label">{field.name}{field.required&&<span aria-label="必填"> *</span>}</label>
    {control}
    {help&&<small>{help}</small>}
  </div>;
}

const systemLabels:Record<SystemFieldCode,string>={
  id:'记录 ID',createdBy:'创建人',createdAt:'创建时间',updatedAt:'更新时间',recordVersion:'记录版本',
};

export type FormPreviewProps = {
  fields: (Field | FieldInput)[];
  layout: (LayoutNode | LayoutNodeInput)[];
  referenceOptions?: {member?:ReferenceOption[];department?:ReferenceOption[]};
};

/** Local-only preview; no HTTP request or record write is performed. */
export function FormPreview({fields,layout,referenceOptions}:FormPreviewProps) {
  const [values,setValues]=useState<Record<UUID,FieldValue>>({});
  const fieldById=new Map(fields.map(field=>[field.id,field]));
  const update=(id:UUID,value:FieldValue)=>setValues(previous=>({...previous,[id]:value}));
  const renderNode=(node:LayoutNode|LayoutNodeInput) => {
    const span=node.kind==='field'||node.kind==='system_field'||node.kind==='group'
      ? node.span??12 : 12;
    const style={gridColumn:`span ${span} / span ${span}`};
    if(node.kind==='divider')return <hr className="forms-divider" key={node.id} style={style}/>;
    if(node.kind==='description')return <p className="forms-description" key={node.id} style={style}>{node.text}</p>;
    if(node.kind==='group')return <fieldset className="forms-preview-group" key={node.id} style={style}>
      <legend>{node.title}</legend><div className="forms-preview-grid">{node.children.map(renderNode)}</div>
    </fieldset>;
    if(node.kind==='system_field')return <div key={node.id} style={style} className="forms-rendered-field">
      <label>{systemLabels[node.fieldId]}</label><input readOnly value="由系统填写" aria-label={systemLabels[node.fieldId]}/>
    </div>;
    const field=fieldById.get(node.fieldId);
    if(!field)return null;
    const options=field.kind==='member'?referenceOptions?.member:
      field.kind==='department'?referenceOptions?.department:undefined;
    return <div key={node.id} style={style}>
      <FieldRenderer field={field} idPrefix="preview" value={Object.hasOwn(values,field.id)?values[field.id]:field.default}
        onChange={value=>update(field.id,value)} referenceOptions={options}/>
    </div>;
  };
  return <div className="forms-preview-grid">{layout.map(renderNode)}</div>;
}
