import type {ReferenceCandidate} from '../forms/FieldRenderer';
export type TaskCandidateData={items:ReferenceCandidate[]};
const object=(value:unknown):value is Record<string,unknown>=>value!==null&&typeof value==='object'&&!Array.isArray(value);
const exact=(value:Record<string,unknown>,keys:string[])=>Object.keys(value).length===keys.length&&keys.every(key=>Object.hasOwn(value,key));
const id=(value:unknown)=>typeof value==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(value)&&value!=='00000000-0000-0000-0000-000000000000';
export function validTaskCandidates(data:unknown,pagination:unknown,kind:'member'|'department',pageSize:number):data is TaskCandidateData {
 if(!Number.isInteger(pageSize)||pageSize<1||pageSize>50||!object(data)||!exact(data,['items'])||!Array.isArray(data.items)||data.items.length>pageSize||!object(pagination)||!exact(pagination,['hasMore','nextPageToken'])||typeof pagination.hasMore!=='boolean')return false;
 if(pagination.hasMore?typeof pagination.nextPageToken!=='string'||!/^[A-Za-z0-9_-]{43}$/.test(pagination.nextPageToken):pagination.nextPageToken!==null)return false;
 const keys=kind==='department'?['id','label','status','parentId']:['id','label','status'];
 if(!data.items.every(item=>object(item)&&exact(item,keys)&&id(item.id)&&typeof item.label==='string'&&item.label.trim()!==''&&item.status==='active'&&(kind!=='department'||item.parentId===null||id(item.parentId))))return false;
 return new Set(data.items.map(item=>item.id)).size===data.items.length;
}
