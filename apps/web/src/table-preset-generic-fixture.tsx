import {useState} from 'react';
import {createRoot} from 'react-dom/client';
import {TablePresetManager,type ResourcePreset,type ResourcePresetRepository} from './TablePresetManager';

const amount='11111111-1111-4111-8111-111111111111';
const title='22222222-2222-4222-8222-222222222222';
const id='33333333-3333-4333-8333-333333333333';
const invalidId='44444444-4444-4444-8444-444444444444';
const state:{items:ResourcePreset[];writes:unknown[]}={items:[
 {id,name:'已失效',version:2,invalid:true,reason:'字段权限已变化'},
 {id:invalidId,name:'可用方案',version:1,invalid:false,filter:{operator:'and',children:[{fieldId:amount,operator:'gt',value:'9007199254740993.01'}]},hiddenColumnIds:[amount]},
],writes:[]};
Object.assign(window,{__resourcePresetFixture:state});
const repository:ResourcePresetRepository={
 async list(){return {items:structuredClone(state.items)};},
 async read(presetId){return structuredClone(state.items.find(item=>item.id===presetId)!);},
 async save(input,existing){state.writes.push({input,existing});const item:ResourcePreset={...input,id:existing?.id??crypto.randomUUID(),version:(existing?.version??0)+1,invalid:false};state.items=[item,...state.items.filter(old=>old.id!==item.id)];return structuredClone(item);},
 async remove(presetId){state.items=state.items.filter(item=>item.id!==presetId);},
};
function Fixture(){const [active,setActive]=useState<ResourcePreset|null>(null),[hidden,setHidden]=useState<string[]>([]);
 const fields=[{id:amount,label:'金额',kind:'money' as const,operators:['eq','neq','gt','gte','lt','lte'] as const},{id:title,label:'标题',kind:'text' as const,operators:['eq','neq'] as const}],columns=[{id:amount,name:'金额'},{id:title,name:'标题'}];
 return <main><TablePresetManager view="resource" resource={{scopeKey:'actor-1:app-1:view-1',fields,columns,repository,refreshDescriptors:async()=>({fields,columns}),onApply:async preset=>{setActive(preset);setHidden(preset?.hiddenColumnIds??[]);return true;}}} active={active} hiddenColumnIds={hidden} onDirty={()=>{}} onUnauthorized={()=>{}}/><output aria-label="fixture-state">{JSON.stringify({active:active?.id??null,hidden,writes:state.writes})}</output></main>;
}
createRoot(document.getElementById('root')!).render(<Fixture/>);
