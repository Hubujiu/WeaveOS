import type { ActivityAction, EventFilterCondition, EventFilterGroup, MemberFilterCondition, MemberFilterGroup } from './query-contracts';

export type QueryView = 'members' | 'events';
export type QueryFilter<V extends QueryView> = V extends 'members' ? MemberFilterGroup : EventFilterGroup;
export type FilterIssue = { path: string; message: string };
export type FilterValidation<V extends QueryView> = { filter?: QueryFilter<V>; issues: FilterIssue[] };

export type FilterField = {
  key: MemberFilterCondition['field'] | EventFilterCondition['field'];
  label: string;
  kind: 'text' | 'enum' | 'boolean' | 'relation' | 'time';
  choices?: readonly {value: string; label: string}[];
};
const actions: readonly ActivityAction[] = ['DEPARTMENT_CREATED','DEPARTMENT_UPDATED','DEPARTMENT_DELETED','IDENTITY_CREATED','IDENTITY_UPDATED','IDENTITY_DELETED','TEMPLATE_CREATED','TEMPLATE_UPDATED','TEMPLATE_DELETED','MEMBER_IDENTITIES_UPDATED','MEMBER_GROUPS_UPDATED','INVITATION_CREATED'];
const actionNames = ['新建部门','更新部门','删除部门','新建身份','更新身份','删除身份','新建模板','更新模板','删除模板','调整成员身份','调整成员分组','创建邀请'];
export const memberFilterFields: readonly FilterField[] = [
  {key:'account',label:'账号',kind:'text'},
  {key:'status',label:'状态',kind:'enum',choices:[{value:'active',label:'正常'},{value:'disabled',label:'停用'}]},
  {key:'departmentIds',label:'部门',kind:'relation'},
  {key:'identityIds',label:'身份',kind:'relation'},
  {key:'personnelManage',label:'人员管理权限',kind:'boolean'},
];
export const eventFilterFields: readonly FilterField[] = [
  {key:'actorAccount',label:'操作者账号',kind:'text'},
  {key:'object',label:'对象',kind:'text'},
  {key:'detail',label:'详情',kind:'text'},
  {key:'action',label:'操作',kind:'enum',choices:actions.map((value,i)=>({value,label:actionNames[i]}))},
  {key:'outcome',label:'结果',kind:'enum',choices:[{value:'success',label:'成功'},{value:'failure',label:'失败'},{value:'error',label:'错误'}]},
  {key:'occurredAt',label:'发生时间',kind:'time'},
];
export const filterOperators = [
  {value:'eq',label:'等于'}, {value:'neq',label:'不等于'},
  {value:'gt',label:'晚于'}, {value:'gte',label:'不早于'},
  {value:'lt',label:'早于'}, {value:'lte',label:'不晚于'},
] as const;
export const MAX_FILTER_LEAVES = 20;
export const MAX_FILTER_DEPTH = 3;
export const MAX_FILTER_BYTES = 16384;
const record = (value: unknown): value is Record<string, unknown> => typeof value === 'object' && value !== null && !Array.isArray(value);
const exactKeys = (value: Record<string, unknown>,keys: readonly string[]) => Object.keys(value).every(k=>keys.includes(k)) && keys.every(k=>Object.hasOwn(value,k));

function validDate(value: string) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if(!match) return false;
  const [year,month,day] = match.slice(1).map(Number);
  if(year < 1 || month < 1 || month > 12 || day < 1 || day > 31) return false;
  const date = new Date(0); date.setUTCFullYear(year,month-1,day);
  return date.getUTCFullYear()===year && date.getUTCMonth()===month-1 && date.getUTCDate()===day;
}
function validTime(value: unknown) {
  if(record(value)) {
    if(!exactKeys(value,['date','timeZone']) || typeof value.date!=='string' || !validDate(value.date) || typeof value.timeZone!=='string') return false;
    try { new Intl.DateTimeFormat('en',{timeZone:value.timeZone}); return true; } catch { return false; }
  }
  if(typeof value !== 'string') return false;
  const match = /^(\d{4}-\d{2}-\d{2})T([01]\d|2[0-3]):([0-5]\d):([0-5]\d)(?:\.\d{1,6})?(Z|[+-](?:[01]\d|2[0-3]):[0-5]\d)$/.exec(value);
  return !!match && validDate(match[1]) && Number.isFinite(Date.parse(value));
}

// This is an editor guard. The server remains authoritative for permissions,
// time boundaries and wire validation. No date is converted into a 24h range.
export function validateQueryFilter<V extends QueryView>(view: V, input: unknown): FilterValidation<V> {
  if(input===undefined) return {issues:[]};
  const issues: FilterIssue[]=[];
  const fields=view==='members' ? memberFilterFields : eventFilterFields;
  const seen=new WeakSet<object>();
  let leaves=0;
  const fail=(path:string,message:string)=>{issues.push({path,message});};
  function visit(node:unknown,path:string,depth:number,root=false): unknown {
    if(!record(node)) {fail(path,'请选择有效的条件或分组');return node;}
    if(seen.has(node)) {fail(path,'筛选不能循环引用');return node;}
    seen.add(node);
    if(Object.hasOwn(node,'children')) {
      if(!exactKeys(node,['operator','children']) || !['and','or'].includes(String(node.operator)) || !Array.isArray(node.children)) {
        fail(path,'分组必须选择 AND 或 OR 并包含条件列表');return node;
      }
      if(depth>MAX_FILTER_DEPTH) {fail(path,'最多支持 3 层分组');return node;}
      if(!root && !node.children.length) fail(path,'分组不能为空，请添加条件或删除分组');
      return {operator:node.operator,children:node.children.map((child,index)=>visit(child,`${path}.${index}`,depth+Number(record(child)&&Object.hasOwn(child,'children'))))};
    }
    leaves++;
    const field=fields.find(f=>f.key===node.field);
    if(!exactKeys(node,['field','operator','value']) || !field) {fail(path,'请选择此视图支持的字段');return node;}
    const equality=node.operator==='eq'||node.operator==='neq';
    if(!equality && !(field.kind==='time' && filterOperators.some(o=>o.value===node.operator))) fail(path,'此字段不支持该比较方式');
    const value=node.value;
    if(value===null) {
      if(!equality || field.kind==='relation') fail(path,'NULL 只可用于单值字段的等于或不等于');
    } else if(field.kind==='text') {
      if(typeof value!=='string') fail(path,'文本条件需要文本值');
    } else if(field.kind==='enum') {
      if(typeof value!=='string'||!field.choices?.some(c=>c.value===value)) fail(path,'请选择有效选项');
    } else if(field.kind==='boolean') {
      if(typeof value!=='boolean') fail(path,'请选择是或否');
    } else if(field.kind==='relation') {
      if(typeof value!=='string'|| !/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(value)) fail(path,'请选择有效的部门或身份');
    } else if(!validTime(value)) fail(path,'请输入有效日期及时区，或含时区且最多 6 位小数的绝对时刻');
    return {field:node.field,operator:node.operator,value:record(value)?{date:value.date,timeZone:value.timeZone}:value};
  }
  if(!record(input)||!Object.hasOwn(input,'children')) return {issues:[{path:'root',message:'根必须为 AND/OR 分组'}]};
  const result=visit(input,'root',1,true);
  if(leaves>MAX_FILTER_LEAVES) fail('root','整个筛选最多 20 个条件');
  if(!issues.length && new TextEncoder().encode(JSON.stringify(result)).byteLength>MAX_FILTER_BYTES) fail('root','筛选超过 16 KiB，请缩短条件');
  if(issues.length) return {issues};
  if(record(result)&&Array.isArray(result.children)&&!result.children.length) return {issues:[]};
  // Depth/shape/typed-field checks above establish the frozen recursive DTO.
  return {issues:[],filter:result as QueryFilter<V>};
}

// Runtime descriptors are supplied only from the authorized RuntimeView. This
// editor guard does not grant access; the server rechecks every field/scope.
export type ResourceFieldKind = 'text' | 'multiline' | 'number' | 'money' | 'date' | 'datetime' | 'single_select' | 'multi_select' | 'boolean' | 'member' | 'department';
export type ResourceFilterField = { id: string; kind: ResourceFieldKind; operators: readonly ('eq' | 'neq' | 'gt' | 'gte' | 'lt' | 'lte')[]; optionIds?: readonly string[] };
export type ResourceFilterCondition = {fieldId:string;operator:'eq'|'neq'|'gt'|'gte'|'lt'|'lte';value:string|boolean|string[]|null};
export type ResourceFilterGroup = {operator:'and'|'or';children:(ResourceFilterGroup|ResourceFilterCondition)[]};
export function validateResourceFilter(fields:readonly ResourceFilterField[],input:unknown):{filter?:ResourceFilterGroup;issues:FilterIssue[]} {
 if(input===null||input===undefined)return {issues:[]};
 const issues:FilterIssue[]=[],seen=new WeakSet<object>();let leaves=0;
 const issue=(path:string,message:string)=>issues.push({path,message});
 const uuid=/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
 function visit(node:unknown,path:string,depth:number):ResourceFilterGroup|ResourceFilterCondition|null{
  if(!record(node)){issue(path,'请选择有效的条件或分组');return null;}
  if(seen.has(node)){issue(path,'筛选不能循环引用');return null;}
  seen.add(node);
  if(Object.hasOwn(node,'children')){
   if(!exactKeys(node,['operator','children'])||!['and','or'].includes(String(node.operator))||!Array.isArray(node.children)||!node.children.length){issue(path,'分组必须为非空 AND 或 OR');return null;}
   if(depth>MAX_FILTER_DEPTH)issue(path,'最多支持 3 层分组');
   const children=node.children.map((child,i)=>visit(child,`${path}.${i}`,depth+(record(child)&&Object.hasOwn(child,'children')?1:0))).filter((child):child is ResourceFilterGroup|ResourceFilterCondition=>child!==null);
   return {operator:node.operator as 'and'|'or',children};
  }
  leaves++;
  if(!exactKeys(node,['fieldId','operator','value'])||typeof node.fieldId!=='string'){issue(path,'请选择有效字段');return null;}
  const field=fields.find(candidate=>candidate.id===node.fieldId);
  if(!field){issue(path,'此字段不可用于当前查询');return null;}
  const operator=node.operator;
  if(!['eq','neq','gt','gte','lt','lte'].includes(String(operator))||!field.operators.includes(operator as ResourceFilterCondition['operator']))issue(path,'此字段不支持该比较方式');
  const value=node.value,equality=operator==='eq'||operator==='neq';
  if(value===null){if(!equality)issue(path,'NULL 只可用于等于或不等于');}
  else if(field.kind==='text'||field.kind==='multiline'){
   if(typeof value!=='string'||!equality)issue(path,'文本仅支持等于或不等于的文本值');
  }else if(field.kind==='number'||field.kind==='money'){
   if(typeof value!=='string'||!/^[-]?\d+(?:\.\d+)?$/.test(value))issue(path,'数字须使用精确十进制字符串');
  }else if(field.kind==='date'){
   if(typeof value!=='string'||!validDate(value))issue(path,'请输入有效日期');
  }else if(field.kind==='datetime'){
   if(typeof value!=='string'||!validTime(value))issue(path,'请输入带明确偏移的时刻');
  }else if(field.kind==='boolean'){
   if(typeof value!=='boolean'||!equality)issue(path,'布尔字段仅支持等于或不等于');
  }else if(field.kind==='single_select'){
   if(typeof value!=='string'||!field.optionIds?.includes(value)||!equality)issue(path,'请选择当前有效选项');
  }else if(field.kind==='multi_select'){
   if(!Array.isArray(value)||!value.every(id=>typeof id==='string'&&field.optionIds?.includes(id))||new Set(value).size!==value.length||!equality)issue(path,'请选择不重复的当前有效选项集合');
  }else if(typeof value!=='string'||!uuid.test(value)||!equality)issue(path,'请选择有效引用');
  return {fieldId:node.fieldId,operator:operator as ResourceFilterCondition['operator'],value:value as ResourceFilterCondition['value']};
 }
 if(!record(input)||!Object.hasOwn(input,'children'))return {issues:[{path:'root',message:'根必须为 AND/OR 分组'}]};
 const filter=visit(input,'root',1);
 if(leaves>MAX_FILTER_LEAVES)issue('root','整个筛选最多 20 个条件');
 if(filter&&new TextEncoder().encode(JSON.stringify(filter)).byteLength>MAX_FILTER_BYTES)issue('root','筛选超过 16 KiB');
 return issues.length?{issues}:{issues:[],filter:filter as ResourceFilterGroup};
}
