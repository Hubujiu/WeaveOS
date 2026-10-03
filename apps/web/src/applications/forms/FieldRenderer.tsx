import { useState } from 'react';
import type {
  Field, FieldInput, LayoutNode, LayoutNodeInput, Option, SystemFieldCode, UUID,
} from './contracts';

export type FieldValue = string | boolean | UUID[] | null;
export type ReferenceOption = { id: UUID; label: string };
export type FieldRendererProps = {
  field: Field | FieldInput;
  value: FieldValue;
  onChange?: (value: FieldValue) => void;
  readOnly?: boolean;
  referenceOptions?: ReferenceOption[];
};

/** Business-field input only. The parent decides when a value is persisted. */
export function FieldRenderer({field,value,onChange,readOnly=false,referenceOptions}:FieldRendererProps) {
  const disabled=readOnly || !onChange;
  const help=field.presentation.helpText;
  const inputId=`preview-${field.id}`;
  let control;
  switch(field.kind) {
    case 'text':
      control=<input id={inputId} type="text" value={typeof value==='string'?value:''}
        maxLength={field.config.maxLength??undefined} disabled={disabled}
        onChange={event=>onChange?.(event.target.value)}/>;
      break;
    case 'multiline':
      control=<textarea id={inputId} value={typeof value==='string'?value:''}
        maxLength={field.config.maxLength??undefined} disabled={disabled}
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
        {field.config.options.map(option=><option key={option.id} value={option.id}>{option.label}</option>)}
      </select>;
      break;
    case 'multi_select': {
      const selected=Array.isArray(value)?value:[];
      control=<div className="forms-multi-options" role="group" aria-label={field.name}>
        {field.config.options.map((option:Option)=><label key={option.id}>
          <input type="checkbox" checked={selected.includes(option.id)} disabled={disabled}
            onChange={event=>onChange?.(event.target.checked
              ? [...selected.filter(id=>id!==option.id),option.id]
              : selected.filter(id=>id!==option.id))}/>{option.label}
        </label>)}
        {!field.config.options.length&&<span>尚无选项</span>}
      </div>;
      break;
    }
    case 'boolean':
      control=<input id={inputId} type="checkbox" checked={value===true} disabled={disabled}
        onChange={event=>onChange?.(event.target.checked)}/>;
      break;
    case 'member': case 'department':
      control=<select id={inputId} value={typeof value==='string'?value:''}
        disabled={disabled||!referenceOptions?.length}
        onChange={event=>onChange?.(event.target.value||null)}>
        <option value="">{referenceOptions?.length?'请选择':'来源尚未接入'}</option>
        {referenceOptions?.map(option=><option key={option.id} value={option.id}>{option.label}</option>)}
      </select>;
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
      <FieldRenderer field={field} value={values[field.id]??field.default}
        onChange={value=>update(field.id,value)} referenceOptions={options}/>
    </div>;
  };
  return <div className="forms-preview-grid">{layout.map(renderNode)}</div>;
}
