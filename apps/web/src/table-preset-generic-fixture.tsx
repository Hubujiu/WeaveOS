import {useState} from 'react';
import {createRoot} from 'react-dom/client';
import {TablePresetManager,type ResourcePreset,type ResourcePresetRepository} from './TablePresetManager';
import {useResourceQuery} from './usePersonnelQuery';

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
const queryRequests:{scope:string;body:unknown}[]=[];
Object.assign(window,{__resourceQueryFixture:queryRequests});
const emptyPage={items:[] as {id:string}[],total:0,page:1,pageSize:20,queryVersion:'',sort:null};
function QueryFixture(){const [scope,setScope]=useState('actor-A:app-1:view-1'),[page,setPage]=useState(1),[filter,setFilter]=useState<string|null>(null);
 const query=useResourceQuery(scope,{page,pageSize:20,filter},emptyPage,async body=>{queryRequests.push({scope,body});return {...emptyPage,items:[{id:scope}],total:50,page:body.page,queryVersion:'token-'+scope};});
 return <section aria-label="query-fixture"><button onClick={()=>setPage(4)}>跳至第4页</button><button onClick={()=>{setFilter('金额大于10');setPage(1);}}>更改条件</button><button onClick={()=>{setPage(1);query.refresh();}}>显式刷新</button><button onClick={()=>{setScope('actor-B:app-1:view-1');setPage(1);}}>切换主体</button><output aria-label="query-state">{JSON.stringify({data:query.data,blocked:query.blocked})}</output></section>;
}
createRoot(document.getElementById('root')!).render(<><Fixture/><QueryFixture/></>);
