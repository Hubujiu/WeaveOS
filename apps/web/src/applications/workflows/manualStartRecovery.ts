import {scopedUnconfirmed,type ApplicationPacket} from '../recovery';
export function findManualStartRecovery(actorId:string,appId:string,viewId:string,recordId:string):ApplicationPacket|null{
 const scope=JSON.stringify(['manual-start',actorId,appId,viewId,recordId]);
 return scopedUnconfirmed(actorId).find(packet=>{
  const resource=packet.resource;
  return packet.actorId===actorId&&resource?.kind==='manual-start'&&resource.appId===appId&&resource.viewId===viewId&&resource.id===recordId&&packet.scope===JSON.stringify([scope,resource]);
 })??null;
}
