import type {Request} from '@playwright/test';

// Existing visual/keyboard oracles now use the approved Q36 POST transport.
// This helper is exclusively a synthetic network fixture, never an app import.
export function fixtureRequestURL(request:Request){
 const url=new URL(request.url());
 if(url.pathname.endsWith('/search')){
  url.pathname=url.pathname.slice(0,-'/search'.length);
  for(const [key,value] of Object.entries(request.postDataJSON() as Record<string,unknown>))if(typeof value==='string'||typeof value==='number')url.searchParams.set(key,String(value));
 }
 return url;
}
const actionNames:Record<string,string>={DEPARTMENT_CREATED:'新建部门',DEPARTMENT_UPDATED:'重命名部门',DEPARTMENT_DELETED:'删除部门',IDENTITY_CREATED:'新建身份',IDENTITY_UPDATED:'修改身份',IDENTITY_DELETED:'删除身份',TEMPLATE_CREATED:'新建权限模板',TEMPLATE_UPDATED:'修改权限模板',TEMPLATE_DELETED:'删除权限模板',MEMBER_IDENTITIES_UPDATED:'分配身份',MEMBER_GROUPS_UPDATED:'调整分组',INVITATION_CREATED:'邀请成员'};
const objectNames:Record<string,string>={department:'部门',identity:'身份',template:'权限模板',member:'成员',invitation:'邀请码'};
const object=(value:unknown):Record<string,unknown>=>value&&typeof value==='object'&&!Array.isArray(value)?value as Record<string,unknown>:{};
export function q36FixtureEnvelope(data:unknown,request:Request){
 const path=new URL(request.url()).pathname;
 if(path.endsWith('/drafts')&&request.method()==='GET')data={items:[]};
 if(path.endsWith('/search')){
  const value=object(data),input=object(request.postDataJSON());
  const items=Array.isArray(value.items)?value.items.map(raw=>{
   const item=object(raw);if(!path.includes('/events/'))return item;
   const before=object(object(item.summary).before),after=object(object(item.summary).after);
   const name=typeof after.name==='string'?after.name:typeof before.name==='string'?before.name:undefined;
   return {...item,display:{action:actionNames[String(item.action)]||'配置变更',object:name||(objectNames[String(item.objectType)]||'操作对象')+(item.objectId?' · '+String(item.objectId).slice(0,8):''),detail:before.name!==after.name?'名称：'+String(before.name||'无')+' → '+String(after.name||'无'):'配置已更新',outcome:item.outcome==='success'?'已完成':'失败'}};
  }):[];
  data={...value,items,queryVersion:'synthetic-q36-context',sort:path.includes('/events/')&&input.sortBy?{key:input.sortBy,direction:input.sortDirection}:null,...(path.includes('/events/')?{range:{from:input.from||'2026-09-24T00:00:00Z',to:input.to||'2026-10-02T00:00:00Z'}}:{})};
 }
 return {code:'OK',message:'success',data,meta:null};
}
