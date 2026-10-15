import {decodeTemplateFile} from './fileInput';
const object=(v:unknown):v is Record<string,unknown>=>!!v&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:string[])=>Object.keys(v).length===keys.length&&keys.every(key=>Object.hasOwn(v,key));
const id=(v:unknown):v is string=>typeof v==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(v)&&v!=='00000000-0000-0000-0000-000000000000';
export function validTemplateImportRequest(resource:unknown,request:unknown):boolean{
 if(!object(resource)||!exact(resource,['kind'])||resource.kind!=='template-import'||!object(request)||request.path!=='application-templates/import'||request.method!=='POST'||request.expectedStatus!==201||!object(request.body)||!exact(request.body,['manifest','bindings'])||!Array.isArray(request.body.bindings))return false;
 try{
  const encode=(v:unknown)=>new TextEncoder().encode(JSON.stringify(v));
  if(encode(request.body).byteLength>4194304||!decodeTemplateFile(encode(request.body.manifest)))return false;
  const seen=new Set<string>();
  return request.body.bindings.every(binding=>{
   if(!object(binding)||!exact(binding,['kind','sourceId','targetId'])||!['user','department'].includes(binding.kind as string)||!id(binding.sourceId)||!id(binding.targetId))return false;
   const key=binding.kind+':'+binding.sourceId;if(seen.has(key))return false;seen.add(key);return true;
  });
 }catch{return false;}
}
