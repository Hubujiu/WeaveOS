import {scopedUnconfirmed,type ApplicationPacket} from '../recovery';
export function findPresetRecovery(actorId:string,appId:string,viewId:string):ApplicationPacket|null{
 const scope=JSON.stringify(['presets',actorId,appId,viewId]);
 return scopedUnconfirmed(actorId).find(packet=>{
  const resource=packet.resource;
  return packet.actorId===actorId&&resource?.kind==='preset'&&resource.appId===appId&&resource.viewId===viewId&&packet.scope===JSON.stringify([scope,resource]);
 })??null;
}
