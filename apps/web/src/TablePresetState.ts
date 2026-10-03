import {validateQueryFilter,validateResourceFilter,type QueryView,type ResourceFilterField,type ResourceFilterGroup} from './QueryFilterState';
import type {MemberFilterGroup,EventFilterGroup} from './query-contracts';

export type PresetFilter=MemberFilterGroup|EventFilterGroup;
export type TablePreset={id:string;view:QueryView;name:string;filter:PresetFilter|null;hiddenColumnIds:string[];schemaVersion:1;version:number;createdAt:string;updatedAt:string};
export type AppliedPreset=Pick<TablePreset,'id'|'version'|'filter'|'hiddenColumnIds'>;
export type PresetChoice={value:string;label:string};
export type PresetOptions={departmentIds?:readonly PresetChoice[];identityIds?:readonly PresetChoice[]};
export type BusinessColumn={id:string;name:string};
export const memberBusinessColumns:readonly BusinessColumn[]=[{id:'account',name:'成员'},{id:'departments',name:'部门'},{id:'identities',name:'身份'},{id:'personnelManage',name:'人员管理'}];
export const eventBusinessColumns:readonly BusinessColumn[]=[{id:'occurredAt',name:'时间'},{id:'actorAccount',name:'操作者'},{id:'action',name:'操作'},{id:'object',name:'对象'},{id:'detail',name:'详情'},{id:'outcome',name:'结果'}];
export type PresetRow={id:string;field:string;operator:string;value:unknown};
export type PresetBlock={id:string;rows:PresetRow[]};
export const newPresetRow=(view:QueryView):PresetRow=>({id:crypto.randomUUID(),field:view==='members'?'account':'actorAccount',operator:'eq',value:''});
const record=(v:unknown):v is Record<string,unknown>=>typeof v==='object'&&v!==null&&!Array.isArray(v);
export function editablePresetFilter(filter:unknown):boolean{
 if(filter===null||filter===undefined)return true;
 const leaf=(v:unknown)=>record(v)&&!Object.hasOwn(v,'children');
 const and=(v:unknown)=>record(v)&&v.operator==='and'&&Array.isArray(v.children)&&v.children.length>0&&v.children.every(leaf);
 return and(filter)||(record(filter)&&filter.operator==='or'&&Array.isArray(filter.children)&&filter.children.length>0&&filter.children.every(and));
}
export function presetBlocks(filter:unknown,key:'field'|'fieldId'='field'):PresetBlock[]{
 if(!record(filter)||!Array.isArray(filter.children)||!filter.children.length)return [];
 const blocks=filter.operator==='and'?[filter]:filter.children;
 return blocks.map(block=>({id:crypto.randomUUID(),rows:record(block)&&Array.isArray(block.children)?block.children.map(row=>({id:crypto.randomUUID(),field:record(row)?String(row[key]??''):'',operator:record(row)?String(row.operator??''):'',value:record(row)?row.value:undefined})):[]}));
}
export function blockFilter(blocks:PresetBlock[],key:'field'|'fieldId'='field'):unknown{
 if(!blocks.length)return undefined;
 const groups=blocks.map(b=>({operator:'and',children:b.rows.map(({field,operator,value})=>({[key]:field,operator,value}))}));
 return groups.length===1?groups[0]:{operator:'or',children:groups};
}
// Application preset wire is pending V013. This validates editor state only;
// the server remains authoritative for live schema, permission and CAS.
export type ResourcePresetValidation={issues:string[];name:string;filter:ResourceFilterGroup|null};
export function validateResourcePreset(name:string,filter:unknown,hidden:readonly string[],fields:readonly ResourceFilterField[],columns:readonly BusinessColumn[]):ResourcePresetValidation{
 const issues:string[]=[],trimmed=name.trim();
 if(!trimmed||[...trimmed].length>100)issues.push('自定义筛选名称须为 1–100 个字符');
 if(/\p{Surrogate}/u.test(trimmed))issues.push('名称包含无效 Unicode 字符');
 if(new Set(hidden).size!==hidden.length||hidden.some(id=>!columns.some(column=>column.id===id)))issues.push('存在已失效显隐字段，请编辑后再应用');
 if(columns.length&&columns.every(column=>hidden.includes(column.id)))issues.push('至少保留一个业务字段');
 const validation=validateResourceFilter(fields,filter);
 for(const problem of validation.issues)issues.push(`条件 ${problem.path}：${problem.message}`);
 const normalized=validation.filter??null;
 if(new TextEncoder().encode(JSON.stringify({name:trimmed,filter:normalized,hiddenColumnIds:hidden})).byteLength>32768)issues.push('方案内容超过 32 KiB，请缩短内容');
 return {issues,name:trimmed,filter:normalized};
}
export function validatePreset(view:QueryView,name:string,filter:unknown,hidden:readonly string[],options:PresetOptions={}){
 const issues:string[]=[];const trimmed=name.trim();
 if(!trimmed||[...trimmed].length>100)issues.push('自定义筛选名称须为 1–100 个字符');
 if(/\p{Surrogate}/u.test(trimmed))issues.push('名称包含无效 Unicode 字符');
 const columns=view==='members'?memberBusinessColumns:eventBusinessColumns;
 if(new Set(hidden).size!==hidden.length||hidden.some(id=>!columns.some(c=>c.id===id)))issues.push('存在已失效显隐字段，请编辑后再应用');
 if(columns.every(c=>hidden.includes(c.id)))issues.push('至少保留一个业务字段');
 const input=filter===null?undefined:filter;
 const validation=validateQueryFilter(view,input);
 if(record(input)&&Array.isArray(input.children)){
  const blocks=input.operator==='and'?[input]:input.operator==='or'?input.children:[];
  if(!blocks.length||blocks.some(b=>!record(b)||b.operator!=='and'||!Array.isArray(b.children)||!b.children.length||b.children.some(r=>!record(r)||Object.hasOwn(r,'children'))))issues.push('仅支持组内且条件、组间或条件；中间分组不能为空');
  blocks.forEach((b,i)=>{if(!record(b)||!Array.isArray(b.children))return;b.children.forEach((r,j)=>{
   if(!record(r))return;const path=`条件 ${i+1}.${j+1}`;
   if(r.value===undefined)issues.push(path+'：请填写或选择值');
   if(r.field==='departmentIds'||r.field==='identityIds'){
    const choices=r.field==='departmentIds'?options.departmentIds:options.identityIds;
    if(!choices?.some(c=>c.value===r.value))issues.push(path+'：已失效引用，请重新选择');
   }
  });});
 }
 for(const issue of validation.issues)issues.push((issue.path==='root'?'筛选':`条件 ${issue.path.slice(5).split('.').map(v=>Number(v)+1).join('.')}`)+'：'+issue.message);
 const normalized=validation.filter??null;
 if(new TextEncoder().encode(JSON.stringify({view,name:trimmed,filter:normalized,hiddenColumnIds:hidden,schemaVersion:1})).length>32768)issues.push('方案内容超过 32 KiB，请缩短内容');
 return {issues,name:trimmed,filter:normalized};
}
