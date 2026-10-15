import type {ReferenceCandidate,ReferenceCandidatePage} from '../forms/FieldRenderer';
const object=(v:unknown):v is Record<string,unknown>=>v!==null&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:string[])=>Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const uuid=(v:unknown):v is string=>typeof v==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(v)&&v!=='00000000-0000-0000-0000-000000000000';
export function templateCandidatePage(value:unknown,kind:'user'|'department'):ReferenceCandidatePage|null{
 if(!object(value)||!object(value.data)||!exact(value.data,['items'])||!Array.isArray(value.data.items)||value.data.items.length>50||!object(value.meta)||!object(value.meta.pagination))return null;
 const page=value.meta.pagination;
 if(!exact(page,['nextPageToken','hasMore'])||typeof page.hasMore!=='boolean')return null;
 const token=page.nextPageToken;
 if(token!==null&&(typeof token!=='string'||!/^[A-Za-z0-9_-]{43}$/.test(token)))return null;
 if(page.hasMore!==(token!==null))return null;
 const seen=new Set<string>(),items:ReferenceCandidate[]=[];
 for(const item of value.data.items){
  if(!object(item)||!exact(item,kind==='user'?['id','label','status']:['id','label','status','parentId'])||!uuid(item.id)||seen.has(item.id)||typeof item.label!=='string'||item.status!=='active')return null;
  if(kind==='department'&&item.parentId!==null&&!uuid(item.parentId))return null;
  seen.add(item.id);
  items.push(kind==='user'?{id:item.id,label:item.label,status:'active'}:{id:item.id,label:item.label,status:'active',parentId:item.parentId as string|null});
 }
 return {items,nextPageToken:token as string|null};
}
