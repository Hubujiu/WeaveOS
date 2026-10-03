// Root-authored HTTP-only fixture for the existing RecordForm regression oracles.
type Wire={operationId:string;values?:Record<string,unknown>;changes?:Record<string,unknown>;[key:string]:unknown};
type Events={saves:Wire[];recoveries:string[];confirmations:unknown[];discarded:number};
export function installRootRecordTransport(events:Events,scenario:string,actor:string,recordId:string){
 const original=window.fetch.bind(window);
 const reply=(status:number,data:unknown,code='OK',headers:Record<string,string>={})=>new Response(JSON.stringify({code,data,meta:null}),{status,headers:{'Content-Type':'application/json',...headers}});
 const receipt=(operationId:string)=>({operationId,id:recordId,recordVersion:1,schemaVersion:2,createdAt:'2026-10-03T09:00:00Z',updatedAt:'2026-10-03T09:00:00Z'});
 window.fetch=async(input,init)=>{
  const url=new URL(typeof input==='string'?input:input instanceof URL?input.href:input.url,location.origin);
  const method=init?.method??'GET';
  if(url.pathname==='/api/v1/sessions/current')return reply(200,{id:actor,account:'root'});
  if(url.pathname.endsWith('/records')&&method==='POST'){
   const wire=JSON.parse(String(init?.body)) as Wire;events.saves.push(wire);
   if(scenario==='initial-failure'&&events.saves.length===1)return reply(400,null,'COMMON_INVALID_REQUEST');
   if(scenario==='initial-failure')return reply(201,receipt(wire.operationId),'OK',{Location:url.pathname+'/'+recordId});
   return reply(503,null,'APPLICATION_OPERATION_UNCONFIRMED');
  }
  if(url.pathname.startsWith('/api/v1/application-operations/')){
   const operationId=url.pathname.split('/').pop()!;events.recoveries.push(operationId);
   if(events.recoveries.length===1&&scenario!=='immediate'){
    if(scenario==='throw')throw new TypeError('Failed to fetch');
    if(scenario==='malformed')return reply(200,{operationId,status:'confirmed',httpStatus:201,result:{operationId}});
    return reply(503,null,'COMMON_SERVICE_UNAVAILABLE');
   }
   const resource='/api/v1/applications/22222222-2222-4222-8222-222222222222/forms/33333333-3333-4333-8333-333333333333/records/'+recordId;
   return reply(200,{operationId,status:'confirmed',httpStatus:201,result:receipt(operationId),location:resource});
  }
  return original(input,init);
 };
}
