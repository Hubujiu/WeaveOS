export type FilterCandidate={id:string;label:string;deleted:boolean};
export type FilterCandidatePage={items:FilterCandidate[];nextPageToken:string|null};
const object=(v:unknown):v is Record<string,unknown>=>v!==null&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:string[])=>Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const uuid=(v:unknown):v is string=>typeof v==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(v)&&v!=='00000000-0000-0000-0000-000000000000';
// Deliberately separate from write candidates: deleted describes a registry
// tombstone; deleted=false is not proof of an active/selectable write account.
export function filterCandidatePage(value:unknown):FilterCandidatePage|null{
 if(!object(value)||!object(value.data)||!exact(value.data,['items'])||!Array.isArray(value.data.items)||value.data.items.length>50||!object(value.meta)||!exact(value.meta,['requestId','pagination'])||typeof value.meta.requestId!=='string'||!object(value.meta.pagination))return null;
 const page=value.meta.pagination;
 if(!exact(page,['nextPageToken','hasMore'])||typeof page.hasMore!=='boolean')return null;
 const token=page.nextPageToken;
 if(token!==null&&(typeof token!=='string'||!/^[A-Za-z0-9_-]{43}$/.test(token)))return null;
 if(page.hasMore!==(token!==null))return null;
 const seen=new Set<string>(),items:FilterCandidate[]=[];
 for(const item of value.data.items){
  if(!object(item)||!exact(item,['id','label','deleted'])||!uuid(item.id)||seen.has(item.id)||typeof item.label!=='string'||typeof item.deleted!=='boolean')return null;
  seen.add(item.id);items.push({id:item.id,label:item.label,deleted:item.deleted});
 }
 return {items,nextPageToken:token as string|null};
}
