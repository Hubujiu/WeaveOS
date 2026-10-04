import type { ApplicationFormSlots } from '../types';

export function FormSlots({ slots }: { slots?: ApplicationFormSlots }) {
 if (!slots?.tabs.length) return <section className="app-surface app-workspace-empty"><h2>尚未配置表单</h2><p>此应用还没有可用表单。</p></section>;
 return <>
  <div className="app-form-tabs" role="tablist" aria-label="应用表单">
   {slots.tabs.map(tab => <button key={tab.id} type="button" role="tab" id={'form-tab-' + tab.id} aria-selected={slots.activeId === tab.id} aria-controls={'form-panel-' + tab.id} onClick={() => slots.select(tab.id)}>{tab.label}{tab.dirty && <span aria-label="有未保存修改"> ·</span>}</button>)}
  </div>
  {slots.tabs.map(tab => <section key={tab.id} role="tabpanel" id={'form-panel-' + tab.id} aria-labelledby={'form-tab-' + tab.id} hidden={slots.activeId !== tab.id}>{tab.content}</section>)}
 </>;
}

