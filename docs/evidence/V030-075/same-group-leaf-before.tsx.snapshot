import type {ResourceFilterCondition,ResourceFilterField,ResourceFilterGroup} from '../../QueryFilterState';

const comparisons={eq:'等于',neq:'不等于',gt:'大于',gte:'大于或等于',lt:'小于',lte:'小于或等于'};
type Props={value:ResourceFilterGroup;fields:ResourceFilterField[];disabled:boolean;onChange:(value:ResourceFilterGroup)=>void;path?:string};
const emptyValue=(field:ResourceFilterField|undefined):ResourceFilterCondition['value']=>field?.kind==='boolean'?false:field?.kind==='multi_select'?[]:'';
// Every callback replaces only the original subtree. Invalid drafts remain visible
// and are rejected by the shared validator, never silently normalized away.
export function WorkflowConditionEditor({value,fields,disabled,onChange,path='1'}:Props){
 const addGroup=(operator:'and'|'or')=>onChange({...value,children:[...value.children,{operator,children:[{fieldId:'',operator:'eq',value:''}]}]});
 return <div className="workflow-condition-group">
  <label>条件组 {path}<select className="admin-input" aria-label={'条件组 '+path+' 的逻辑'} value={value.operator} disabled={disabled} onChange={e=>onChange({...value,operator:e.target.value as 'and'|'or'})}><option value="and">且（全部满足）</option><option value="or">或（任一满足）</option></select></label>
  {value.children.map((child,index)=>{
   const childPath=path+'.'+(index+1),change=(next:ResourceFilterGroup|ResourceFilterCondition)=>onChange({...value,children:value.children.map((existing,i)=>i===index?next:existing)});
   const remove=()=>onChange({...value,children:value.children.filter((_,i)=>i!==index)});
   if('children' in child)return <div key={index}><WorkflowConditionEditor value={child} fields={fields} disabled={disabled} onChange={change} path={childPath}/><button type="button" className="admin-button" aria-label={'删除条件组 '+childPath} disabled={disabled} onClick={remove}>删除条件组</button></div>;
   const field=fields.find(f=>f.id===child.fieldId),canEdit=!!field&&!disabled,valueLabel='条件 '+childPath+' 的值';
   return <div className="workflow-condition-leaf" key={index}>
    <label>字段<select className="admin-input" aria-label={'条件 '+childPath+' 的字段'} value={child.fieldId} disabled={disabled} onChange={e=>{const next=fields.find(f=>f.id===e.target.value);change({...child,fieldId:e.target.value,operator:'eq',value:emptyValue(next)});}}><option value="">选择字段</option>{!field&&child.fieldId&&<option value={child.fieldId}>字段已不可用：{child.fieldId}</option>}{fields.map(f=><option key={f.id} value={f.id}>{f.label??f.id}</option>)}</select></label>
    <label>比较<select className="admin-input" aria-label={'条件 '+childPath+' 的比较'} value={child.operator} disabled={!canEdit} onChange={e=>change({...child,operator:e.target.value as ResourceFilterCondition['operator']})}>{field?.operators.map(operator=><option key={operator} value={operator}>{comparisons[operator]}</option>)}</select></label>
    <label className="workflow-condition-null"><input type="checkbox" aria-label={'条件 '+childPath+' 使用 NULL'} checked={child.value===null} disabled={!canEdit} onChange={e=>change({...child,value:e.target.checked?null:emptyValue(field)})}/>使用 NULL（无值）</label>
    {child.value===null?<p>NULL</p>:<div className="workflow-condition-value"><span>值</span>{field?.kind==='single_select'?<select className="admin-input" aria-label={valueLabel} value={typeof child.value==='string'?child.value:''} disabled={!canEdit} onChange={e=>change({...child,value:e.target.value})}><option value="">选择选项</option>{field.choices?.map(option=><option value={option.value} key={option.value}>{option.label}</option>)}</select>:field?.kind==='multi_select'?<span role="group" aria-label={valueLabel}>{field.choices?.map(option=><label key={option.value}><input type="checkbox" checked={Array.isArray(child.value)&&child.value.includes(option.value)} disabled={!canEdit} onChange={e=>{const values=Array.isArray(child.value)?child.value:[];change({...child,value:e.target.checked?[...values,option.value]:values.filter(id=>id!==option.value)});}}/>{option.label}</label>)}</span>:field?.kind==='boolean'?<select className="admin-input" aria-label={valueLabel} value={String(child.value)} disabled={!canEdit} onChange={e=>change({...child,value:e.target.value==='true'})}><option value="false">否</option><option value="true">是</option></select>:typeof child.value==='string'&&field&&['text','multiline','number','money','date','datetime'].includes(field.kind)?<input className="admin-input" aria-label={valueLabel} value={child.value} disabled={!canEdit} onChange={e=>change({...child,value:e.target.value})}/>:<span>{Array.isArray(child.value)?child.value.join('、'):String(child.value)}</span>}</div>}
    <button type="button" className="admin-button" aria-label={'删除条件 '+childPath} disabled={disabled} onClick={remove}>删除条件</button>
   </div>;
  })}
  <div className="workflow-condition-actions"><button type="button" className="admin-button" aria-label={'条件组 '+path+' 添加且条件'} disabled={disabled||path.split('.').length>=3} onClick={()=>addGroup('and')}>＋ 且条件</button><button type="button" className="admin-button" aria-label={'条件组 '+path+' 添加或条件'} disabled={disabled||path.split('.').length>=3} onClick={()=>addGroup('or')}>＋ 或条件</button></div>
 </div>;
}
