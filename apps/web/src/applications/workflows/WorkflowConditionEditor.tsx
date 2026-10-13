import type {ResourceFilterCondition,ResourceFilterField,ResourceFilterGroup} from '../../QueryFilterState';

const comparisons={eq:'等于',neq:'不等于',gt:'大于',gte:'大于或等于',lt:'小于',lte:'小于或等于'};
type Props={value:ResourceFilterGroup;fields:ResourceFilterField[];disabled:boolean;onChange:(value:ResourceFilterGroup)=>void;path?:string};
// Edit each original subtree in place; never flatten an existing expression.
export function WorkflowConditionEditor({value,fields,disabled,onChange,path='1'}:Props){
 return <div className="workflow-condition-group"><label>条件组 {path}<select className="admin-input" aria-label={'条件组 '+path+' 的逻辑'} value={value.operator} disabled={disabled} onChange={e=>onChange({...value,operator:e.target.value as 'and'|'or'})}><option value="and">且（全部满足）</option><option value="or">或（任一满足）</option></select></label>
 {value.children.map((child,index)=>{const childPath=path+'.'+(index+1);const change=(next:ResourceFilterGroup|ResourceFilterCondition)=>onChange({...value,children:value.children.map((existing,i)=>i===index?next:existing)});
 if('children' in child)return <WorkflowConditionEditor key={index} value={child} fields={fields} disabled={disabled} onChange={change} path={childPath}/>;
 const field=fields.find(f=>f.id===child.fieldId),canEdit=!!field&&!disabled;
 return <div className="workflow-condition-leaf" key={index}><p>{field?.label??'字段已不可用：'+child.fieldId}</p><label>比较<select className="admin-input" aria-label={'条件 '+childPath+' 的比较'} value={child.operator} disabled={!canEdit} onChange={e=>change({...child,operator:e.target.value as ResourceFilterCondition['operator']})}>{field?.operators.map(operator=><option key={operator} value={operator}>{comparisons[operator]}</option>)}</select></label><label>值{typeof child.value==='string'&&field&&['text','multiline','number','money','date','datetime'].includes(field.kind)?<input className="admin-input" aria-label={'条件 '+childPath+' 的值'} value={child.value} disabled={!canEdit} onChange={e=>change({...child,value:e.target.value})}/>:<span>{child.value===null?'NULL':typeof child.value==='boolean'?(child.value?'是':'否'):Array.isArray(child.value)?child.value.join('、'):child.value}</span>}</label></div>;
 })}</div>;
}
