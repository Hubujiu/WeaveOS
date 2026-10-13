import {decodeTemplateFile} from './fileInput';
export type TemplateReferences={userIds:string[];departmentIds:string[]};
const object=(v:unknown):v is Record<string,unknown>=>!!v&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:string[])=>Object.keys(v).length===keys.length&&keys.every(key=>Object.hasOwn(v,key));
const id=(v:unknown):v is string=>typeof v==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(v)&&v!=='00000000-0000-0000-0000-000000000000';
// Discover only IDs carried by the file. Current target authority and the
// complete field/layout/graph semantics remain the server preflight's job.
export function templateReferences(manifest:unknown):TemplateReferences|null{
 try{
  const m=decodeTemplateFile(new TextEncoder().encode(JSON.stringify(manifest)));if(!m)return null;
  const users=new Set<string>(),departments=new Set<string>();
  const add=(kind:string,value:unknown):boolean=>{
   if(value===null)return true;
   if(!id(value))return false;
   (kind==='member'?users:departments).add(value);return true;
  };
  const list=(value:unknown):boolean=>Array.isArray(value)&&value.every(v=>id(v)&&add('member',v));
  const tables=new Map<string,Map<string,string>>(),allFields=new Set<string>();
  const kinds=['text','multiline','number','boolean','date','datetime','single_select','multi_select','member','department'];
  for(const table of m.tables as unknown[]){
   if(!object(table)||!id(table.id)||tables.has(table.id)||!Array.isArray(table.fields))return null;
   const fields=new Map<string,string>();tables.set(table.id,fields);
   for(const field of table.fields){
    if(!object(field)||!id(field.id)||allFields.has(field.id)||typeof field.kind!=='string'||!kinds.includes(field.kind)||!Object.hasOwn(field,'default'))return null;
    allFields.add(field.id);fields.set(field.id,field.kind);
    if(['member','department'].includes(field.kind)&&!add(field.kind,field.default))return null;
   }
  }
  const condition=(root:unknown,fields:Map<string,string>):boolean=>{
   if(root===null)return true;
   let leaves=0;
   const visit=(v:unknown,depth:number):boolean=>{
    if(!object(v)||depth>3)return false;
    if(Object.hasOwn(v,'children'))return exact(v,['operator','children'])&&['and','or'].includes(v.operator as string)&&Array.isArray(v.children)&&v.children.length>0&&v.children.length<=20&&v.children.every(child=>visit(child,depth+(object(child)&&Object.hasOwn(child,'children')?1:0)));
    if(++leaves>20||!exact(v,['fieldId','operator','value'])||typeof v.fieldId!=='string'||!['eq','neq','gt','gte','lt','lte'].includes(v.operator as string))return false;
    const kind=fields.get(v.fieldId);
    if(!kind&&!['createdAt','updatedAt'].includes(v.fieldId))return false;
    if(kind==='member'||kind==='department')return add(kind,v.value);
    return v.value===null||typeof v.value==='string'||typeof v.value==='boolean'||Array.isArray(v.value)&&v.value.every(item=>typeof item==='string');
   };
   return object(root)&&Object.hasOwn(root,'children')&&visit(root,1);
  };
  for(const group of m.permissionGroups as unknown[])if(!object(group)||!list(group.memberIds))return null;
  for(const workflow of m.workflows as unknown[]){
   if(!object(workflow)||!id(workflow.tableId))return null;
   const fields=tables.get(workflow.tableId);
   if(!fields||!object(workflow.graph)||!Array.isArray(workflow.graph.nodes)||!Array.isArray(workflow.triggers))return null;
   for(const node of workflow.graph.nodes){
    if(!object(node)||!id(node.id))return null;
    if(node.kind==='start'||node.kind==='end'){if(!exact(node,['id','kind']))return null;}
    else if(node.kind==='approval'){
     if(!exact(node,['id','kind','approval'])||!object(node.approval)||!exact(node.approval,['mode','assigneeIds','editableFieldIds'])||!list(node.approval.assigneeIds))return null;
    }else if(node.kind==='condition'){if(!exact(node,['id','kind','condition'])||node.condition===null||!condition(node.condition,fields))return null;}
    else return null;
   }
   for(const trigger of workflow.triggers)if(!object(trigger)||!exact(trigger,['event','condition'])||!['manual','record.created','record.updated'].includes(trigger.event as string)||!condition(trigger.condition,fields))return null;
  }
  return {userIds:[...users].sort(),departmentIds:[...departments].sort()};
 }catch{return null;}
}
