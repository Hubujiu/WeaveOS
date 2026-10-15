import {scopedUnconfirmed,type ApplicationPacket} from '../recovery';
import {validRoundReworkRequest,validRoundStartRequest} from './roundContract';
export function findRoundRecovery(actorId:string,appId:string,viewId:string,recordId:string,instanceId:string,kind:'round-start'|'round-record'):ApplicationPacket|null{
 const scope=kind==='round-start'?JSON.stringify(['workflow-round',actorId,appId,viewId,recordId,instanceId]):`round-record:${appId}:${viewId}:${recordId}:${instanceId}`;
 return scopedUnconfirmed(actorId).find(packet=>{
  const resource=packet.resource;
  if(packet.actorId!==actorId||resource?.kind!==kind||resource.appId!==appId||resource.viewId!==viewId||resource.id!==recordId||resource.instanceId!==instanceId||packet.scope!==JSON.stringify([scope,resource])||!packet.operationId||packet.body?.operationId!==packet.operationId)return false;
  const {operationId,...body}=packet.body;
  const request={path:packet.path,method:packet.method,expectedStatus:packet.expectedStatus,body};
  return kind==='round-start'?validRoundStartRequest(resource,request):validRoundReworkRequest(resource,request);
 })??null;
}
