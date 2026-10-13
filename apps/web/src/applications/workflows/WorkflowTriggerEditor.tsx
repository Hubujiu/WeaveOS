import type {ResourceFilterField,ResourceFilterGroup} from '../../QueryFilterState';
import type {WorkflowDefinition} from './definitionContract';
import {WorkflowConditionEditor} from './WorkflowConditionEditor';
const events=[['record.created','记录新增'],['record.updated','记录修改'],['manual','手动发起']] as const;
export function WorkflowTriggerEditor({value,fields,disabled,onChange}:{value:WorkflowDefinition['triggers'];fields:ResourceFilterField[];disabled:boolean;onChange:(value:WorkflowDefinition['triggers'])=>void}){
 return <section className="app-surface workflow-trigger-settings"><h2>触发方式</h2>{events.map(([event,label])=><label className="workflow-definition-withdraw" key={event}><input type="checkbox" checked={value.some(t=>t.event===event)} disabled={disabled} onChange={e=>onChange(e.target.checked?[...value,{event,condition:null}]:value.filter(t=>t.event!==event))}/>{label}</label>)}<h2>触发条件</h2>{value.length?value.map(trigger=><section aria-label={events.find(([event])=>event===trigger.event)![1]+'触发条件'} key={trigger.event}><h3>{events.find(([event])=>event===trigger.event)![1]}</h3>{trigger.condition===null?<p>无附加条件</p>:<WorkflowConditionEditor value={trigger.condition as ResourceFilterGroup} fields={fields} disabled={disabled} onChange={condition=>onChange(value.map(t=>t.event===trigger.event?{...t,condition}:t))}/>}</section>):<p>未配置触发方式</p>}</section>;
}
