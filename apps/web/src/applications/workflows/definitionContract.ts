export type WorkflowGraph={version:1;nodes:WorkflowNode[];edges:{from:string;to:string;branch?:'true'|'false'}[]};
export type WorkflowNode=({id:string;kind:'start'|'end'}|{id:string;kind:'approval';approval:{mode:'all'|'any';assigneeIds:string[];editableFieldIds:string[]}}|{id:string;kind:'condition';condition:unknown})&{name?:string};
export type WorkflowDefinition={id:string;name:string;revision:number;currentVersion:number;candidateVersion:number;state:'disabled'|'enabled'|'closing';schemaVersion:number;allowWithdraw:boolean;graph:WorkflowGraph;triggers:{event:'manual'|'record.created'|'record.updated';condition:unknown}[]};
const object=(v:unknown):v is Record<string,unknown>=>!!v&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:string[])=>Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const integer=(v:unknown,min:number):v is number=>typeof v==='number'&&Number.isSafeInteger(v)&&v>=min;
const uuid=(v:unknown):v is string=>typeof v==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(v)&&v!=='00000000-0000-0000-0000-000000000000';
const ids=(v:unknown,min:number,max:number):v is string[]=>Array.isArray(v)&&v.length>=min&&v.length<=max&&v.every(uuid)&&new Set(v).size===v.length;
function condition(value:unknown):boolean{
 let leaves=0;const seen=new WeakSet<object>();
 const visit=(v:unknown,depth:number):boolean=>{
  if(!object(v)||seen.has(v))return false;seen.add(v);
  if(Object.hasOwn(v,'children'))return depth<=3&&exact(v,['operator','children'])&&['and','or'].includes(v.operator as string)&&Array.isArray(v.children)&&v.children.length>0&&v.children.length<=20&&v.children.every(c=>visit(c,depth+(object(c)&&Object.hasOwn(c,'children')?1:0)));
  leaves++;return exact(v,['fieldId','operator','value'])&&uuid(v.fieldId)&&['eq','neq','gt','gte','lt','lte'].includes(v.operator as string)&&(v.value===null||typeof v.value==='string'||typeof v.value==='boolean'||Array.isArray(v.value)&&v.value.every(x=>typeof x==='string'));
 };
 if(!object(value)||!Object.hasOwn(value,'children')||!visit(value,1)||leaves>20)return false;
 return new TextEncoder().encode(JSON.stringify(value)).byteLength<=16384;
}
export function validWorkflowNodeName(value:unknown):value is string{return typeof value==='string'&&(value===''||/[^\p{White_Space}]/u.test(value))&&!value.includes('\0')&&Array.from(value).length<=100&&!Array.from(value).some(c=>{const n=c.codePointAt(0)!;return n>=0xd800&&n<=0xdfff;});}
export function validWorkflowGraph(value:unknown):value is WorkflowGraph{
 if(!object(value)||!exact(value,['version','nodes','edges'])||value.version!==1||!Array.isArray(value.nodes)||value.nodes.length<2||value.nodes.length>100||!Array.isArray(value.edges)||value.edges.length<1||value.edges.length>200)return false;
 const nodes=new Map<string,WorkflowNode>();let starts=0,ends=0;
 for(const n of value.nodes){
  if(!object(n)||!uuid(n.id)||nodes.has(n.id))return false;
  const metadata=Object.hasOwn(n,'name')?['name']:[];if(metadata.length&&!validWorkflowNodeName(n.name))return false;
  if(n.kind==='start'||n.kind==='end'){if(!exact(n,['id','kind',...metadata]))return false;if(n.kind==='start')starts++;else ends++;}
  else if(n.kind==='approval'){
   if(!exact(n,['id','kind','approval',...metadata])||!object(n.approval)||!exact(n.approval,['mode','assigneeIds','editableFieldIds'])||!['all','any'].includes(n.approval.mode as string)||!ids(n.approval.assigneeIds,1,50)||!ids(n.approval.editableFieldIds,0,200))return false;
  }else if(n.kind==='condition'){if(!exact(n,['id','kind','condition',...metadata])||!condition(n.condition))return false;}
  else return false;
  nodes.set(n.id,n as WorkflowNode);
 }
 if(starts!==1||ends<1)return false;
 const incoming=new Map([...nodes.keys()].map(id=>[id,0])),outgoing=new Map([...nodes.keys()].map(id=>[id,[] as string[]])),branches=new Map<string,Set<string>>(),seen=new Set<string>();
 for(const e of value.edges){
  if(!object(e)||!(exact(e,['from','to'])||exact(e,['from','to','branch']))||typeof e.from!=='string'||typeof e.to!=='string'||!nodes.has(e.from)||!nodes.has(e.to)||e.from===e.to)return false;
  const branch=e.branch??'',key=JSON.stringify([e.from,e.to,branch]);if(seen.has(key))return false;seen.add(key);
  if(nodes.get(e.from)!.kind==='condition'){if(branch!=='true'&&branch!=='false')return false;const b=branches.get(e.from)??new Set<string>();if(b.has(branch))return false;b.add(branch);branches.set(e.from,b);}
  else if(Object.hasOwn(e,'branch'))return false;
  incoming.set(e.to,incoming.get(e.to)!+1);outgoing.get(e.from)!.push(e.to);
 }
 for(const [id,n]of nodes){const count=incoming.get(id)!,out=outgoing.get(id)!.length;
  if(n.kind==='start'?(count!==0||out!==1):n.kind==='end'?(count<1||out!==0):n.kind==='approval'?(count<1||out!==1):(count<1||out!==2||branches.get(id)?.size!==2))return false;
 }
 const queue=[...nodes.keys()].filter(id=>incoming.get(id)===0);let visited=0;
 for(let i=0;i<queue.length;i++){visited++;for(const target of outgoing.get(queue[i])!){const next=incoming.get(target)!-1;incoming.set(target,next);if(next===0)queue.push(target);}}
 return visited===nodes.size;
}
export function validWorkflowDefinition(value:unknown,flowId:string):value is WorkflowDefinition{
 if(!uuid(flowId)||!object(value)||!exact(value,['id','name','revision','currentVersion','candidateVersion','state','schemaVersion','allowWithdraw','graph','triggers'])||value.id!==flowId)return false;
 if(typeof value.name!=='string'||!value.name.trim()||[...value.name].length>100||!integer(value.revision,1)||!integer(value.schemaVersion,1)||!integer(value.currentVersion,0)||!integer(value.candidateVersion,1)||value.candidateVersion<value.currentVersion||!['disabled','enabled','closing'].includes(value.state as string)||typeof value.allowWithdraw!=='boolean'||!validWorkflowGraph(value.graph))return false;
 return validWorkflowTriggers(value.triggers);
}
export function validWorkflowTriggers(value:unknown):value is WorkflowDefinition['triggers']{
 if(!Array.isArray(value)||value.length>3||!value.every(t=>object(t)&&exact(t,['event','condition'])&&['manual','record.created','record.updated'].includes(t.event as string)&&(t.condition===null||condition(t.condition))))return false;
 return new Set(value.map(t=>t.event)).size===value.length;
}
