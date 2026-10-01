// Cookie-backed application requests; no browser token storage.
export class WorkspaceError extends Error {
 constructor(readonly status:number,readonly code:string,readonly reason?:string) {super(code==='COMMON_QUERY_CHANGED'?'查询结果已变化，请刷新查询后继续':code==='COMMON_QUERY_CONTEXT_EXPIRED'?'查询上下文已过期，请重新查询后继续':reason==='QUERY_BUSY'?'查询暂时繁忙，请稍后重试':code==='PERSONNEL_DRAFT_CONFLICT'?'草稿已由其他标签页修改，请明确选择后继续':code==='PERSONNEL_DRAFT_LIMIT_REACHED'?'已保存 20 份草稿，请先明确删除不需要的草稿':status===401?'登录已失效，请重新登录':status===403?'没有人员管理权限':status===409?'配置已变更，请重新加载后再编辑':status===400?'输入信息不合法，请检查后重试':status===404?'对象不存在，可能已被删除':'服务暂时不可用，请稍后重试');}
}
export async function workspaceApi<T>(path:string,method='GET',body?:object,signal?:AbortSignal):Promise<T>{
 const headers:Record<string,string>={};
 if(body)headers['Content-Type']='application/json';
 if(!['GET','HEAD'].includes(method)){
  const csrf=document.cookie.split(';').map(v=>v.trim()).find(v=>v.startsWith('__Host-csrf='));
  if(csrf)headers['X-CSRF-Token']=csrf.slice('__Host-csrf='.length);
 }
 let response:Response;
 try{response=await fetch('/api/v1/'+path,{method,credentials:'include',headers,body:body?JSON.stringify(body):undefined,signal});}catch(e){if(signal?.aborted)throw e;throw new WorkspaceError(0,'COMMON_UNAVAILABLE');}
 if(response.status===204)return undefined as T;
 let envelope:{code:string;data:T;meta?:{reason?:string}|null};
 try{envelope=await response.json();}catch{throw new WorkspaceError(response.ok?503:response.status,'COMMON_UNAVAILABLE');}
 if(!response.ok||envelope.code!=='OK')throw new WorkspaceError(response.status,envelope.code,envelope.meta?.reason);
 return envelope.data;
}
