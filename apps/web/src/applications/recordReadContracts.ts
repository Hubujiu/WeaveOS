import type {FieldKind,RecordItem,RecordPage,RecordSort,RuntimeField,RuntimeLayoutNode,RuntimeView,Scope,UUID} from './records/contracts';

const uuid=/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const object=(v:unknown):v is Record<string,any>=>!!v&&typeof v==='object'&&!Array.isArray(v);
const text=(v:unknown):v is string=>typeof v==='string';
const version=(v:unknown):v is number=>typeof v==='number'&&Number.isSafeInteger(v)&&v>=0;
const positiveVersion=(v:unknown):v is number=>version(v)&&v>=1;
const timestamp=(v:unknown):v is string=>text(v)&&/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,9})?(?:Z|[+-]\d\d:\d\d)$/.test(v)&&Number.isFinite(Date.parse(v));
const fail=():never=>{throw new Error('记录响应格式不正确');};
const scopes:Scope[]=['none','own','all'];
const kinds:FieldKind[]=['text','multiline','number','money','date','datetime','single_select','multi_select','boolean','member','department'];
const decimalModes=['HALF_UP','HALF_EVEN','TOWARD_ZERO','FLOOR','CEILING'];

export function parseRuntimeView(raw:unknown,scope:{appId:string;viewId:string}):RuntimeView{
 if(!object(raw)||!uuid.test(scope.appId)||!uuid.test(scope.viewId)||raw.appId!==scope.appId||raw.viewId!==scope.viewId||!uuid.test(raw.tableId)||!positiveVersion(raw.schemaVersion)||!version(raw.viewVersion)||!positiveVersion(raw.policyRevision)||!Array.isArray(raw.fields)||!Array.isArray(raw.layout)||!object(raw.capabilities))return fail();
 const fields:RuntimeField[]=raw.fields.map((f:unknown)=>{
  if(!object(f)||!uuid.test(f.id)||!text(f.name)||!f.name.trim()||!kinds.includes(f.kind)||typeof f.required!=='boolean'||!object(f.presentation)||!(f.presentation.helpText===null||text(f.presentation.helpText))||!(f.presentation.displayTimeZone===null||text(f.presentation.displayTimeZone))||!object(f.input)||!object(f.access)||!object(f.query))return fail();
  if(!scopes.includes(f.access.read)||typeof f.access.create!=='boolean'||!scopes.includes(f.access.edit)||!scopes.includes(f.access.history)||!Array.isArray(f.query.operators)||!f.query.operators.every((op:unknown)=>text(op)&&['eq','neq','gt','gte','lt','lte'].includes(op))||typeof f.query.sortable!=='boolean'||typeof f.query.quickSearchable!=='boolean')return fail();
  if(f.input.options!==undefined&&(!Array.isArray(f.input.options)||!f.input.options.every((option:unknown)=>object(option)&&uuid.test(option.id)&&text(option.label))))return fail();
  if(f.default!==undefined&&!validValue(f.kind,f.default))return fail();
  if(f.input.decimal!==undefined){const d=f.input.decimal;if(!object(d)||!Number.isInteger(d.precision)||d.precision<1||d.precision>38||!Number.isInteger(d.scale)||d.scale<0||d.scale>18||d.scale>d.precision||!Number.isInteger(d.roundingPlaces)||d.roundingPlaces< -18||d.roundingPlaces>18||d.roundingPlaces>d.scale||!decimalModes.includes(d.roundingMode))return fail();}
  if(f.input.timePrecision!==undefined&&!['minute','second','millisecond'].includes(f.input.timePrecision))return fail();
  if(f.input.referenceKind!==undefined&&!['member','department'].includes(f.input.referenceKind))return fail();
  return f as RuntimeField;
 });
 if(new Set(fields.map(f=>f.id)).size!==fields.length)return fail();
 const ids=new Set(fields.map(f=>f.id));
 const layout=(nodes:unknown[]):RuntimeLayoutNode[]=>nodes.map((n:unknown)=>{
  if(!object(n)||!uuid.test(n.id)||!validSpan(n.span))return fail();
  if(n.kind==='field'){if(!uuid.test(n.fieldId)||!ids.has(n.fieldId))return fail();return n as RuntimeLayoutNode;}
  if(n.kind==='system_field'){if(!text(n.fieldId))return fail();return n as RuntimeLayoutNode;}
  if(n.kind==='description'){if(!text(n.text))return fail();return n as RuntimeLayoutNode;}
  if(n.kind==='divider')return n as RuntimeLayoutNode;
  if(n.kind==='group'&&text(n.title)&&Array.isArray(n.children))return {...n,children:layout(n.children)} as RuntimeLayoutNode;
  return fail();
 });
 const c=raw.capabilities;if(typeof c.create!=='boolean'||!scopes.includes(c.read)||!scopes.includes(c.edit)||!scopes.includes(c.history)||typeof c.search!=='boolean'||typeof c.draftCreate!=='boolean'||typeof c.draftEdit!=='boolean')return fail();
 return {...raw,fields,layout:layout(raw.layout)} as RuntimeView;
}
function validSpan(value:unknown):boolean{return value===undefined||Number.isInteger(value)&&typeof value==='number'&&value>=1&&value<=12;}
function calendarDate(value:unknown):value is string{
 if(!text(value))return false;const m=/^(\d{4})-(\d\d)-(\d\d)$/.exec(value);if(!m)return false;
 const year=Number(m[1]),month=Number(m[2]),day=Number(m[3]);const date=new Date(Date.UTC(year,month-1,day));
 return date.getUTCFullYear()===year&&date.getUTCMonth()===month-1&&date.getUTCDate()===day;
}
function validValue(kind:FieldKind,value:unknown):boolean{
 if(value===null)return true;
 if(kind==='boolean')return typeof value==='boolean';
 if(kind==='multi_select')return Array.isArray(value)&&value.every(v=>text(v)&&uuid.test(v));
 if(kind==='member'||kind==='department')return text(value)&&uuid.test(value);
 if(kind==='single_select')return text(value)&&uuid.test(value);
 if(!text(value))return false;
 if(kind==='number'||kind==='money')return /^-?(?:0|[1-9]\d*)(?:\.\d+)?$/.test(value);
 if(kind==='date')return calendarDate(value);
 if(kind==='datetime')return timestamp(value);
 return true;
}
function canReadField(field:RuntimeField,createdBy:string,actorId:string):boolean{return field.access.read==='all'||field.access.read==='own'&&createdBy===actorId;}
export function parseRecordItem(raw:unknown,view:RuntimeView,actorId:string):RecordItem{
 if(!object(raw)||!uuid.test(actorId)||!uuid.test(raw.id)||raw.appId!==view.appId||raw.tableId!==view.tableId||raw.viewId!==view.viewId||!uuid.test(raw.createdBy)||!timestamp(raw.createdAt)||!timestamp(raw.updatedAt)||!positiveVersion(raw.recordVersion)||!positiveVersion(raw.schemaVersion)||raw.schemaVersion!==view.schemaVersion||!object(raw.values)||!object(raw.referenceDisplays))return fail();
 if(view.capabilities.read==='none'||view.capabilities.read==='own'&&raw.createdBy!==actorId)return fail();
 const fieldMap=new Map(view.fields.map(field=>[field.id,field]));
 for(const [id,value] of Object.entries(raw.values)){const field=fieldMap.get(id);if(!field||!canReadField(field,raw.createdBy,actorId)||!validValue(field.kind,value))return fail();}
 for(const [fieldId,displays] of Object.entries(raw.referenceDisplays)){
  const field=fieldMap.get(fieldId);if(!field||!['member','department'].includes(field.kind)||!canReadField(field,raw.createdBy,actorId)||!object(displays))return fail();
  for(const [id,d] of Object.entries(displays))if(!uuid.test(id)||!object(d)||d.id!==id||!text(d.label)||typeof d.deleted!=='boolean')return fail();
 }
 return raw as RecordItem;
}
export function parseRecordPage(raw:unknown,view:RuntimeView,actorId:string,request:{page:number;pageSize:number;sort:RecordSort}):RecordPage{
 if(!object(raw)||!Array.isArray(raw.items)||!version(raw.total)||raw.page!==request.page||raw.pageSize!==request.pageSize||!version(raw.page)||raw.page<1||!Number.isInteger(raw.pageSize)||raw.pageSize<1||raw.pageSize>100||raw.items.length>raw.pageSize||typeof raw.queryVersion!=='string'||!raw.queryVersion||raw.queryVersion.length>512||raw.schemaVersion!==view.schemaVersion||raw.viewVersion!==view.viewVersion)return fail();
 const sort=parseSort(raw.sort,view);if(!sameSort(sort,request.sort))return fail();
 const items=raw.items.map(item=>parseRecordItem(item,view,actorId));
 if(new Set(items.map(item=>item.id)).size!==items.length)return fail();
 return {...raw,items,sort} as RecordPage;
}
function parseSort(raw:unknown,view:RuntimeView):RecordSort{
 if(raw===null)return null;
 if(!object(raw)||!['asc','desc'].includes(raw.direction))return fail();
 if(raw.fieldId==='createdAt'||raw.fieldId==='updatedAt')return {fieldId:raw.fieldId,direction:raw.direction};
 if(!uuid.test(raw.fieldId))return fail();
 const field=view.fields.find(candidate=>candidate.id===raw.fieldId);
 if(!field||!field.query.sortable||field.access.read==='none')return fail();
 return {fieldId:raw.fieldId,direction:raw.direction};
}
function sameSort(a:RecordSort,b:RecordSort):boolean{return a===null||b===null?a===b:a.fieldId===b.fieldId&&a.direction===b.direction;}
