import {scopedUnconfirmed,type ApplicationPacket} from '../recovery';
const id=(value:unknown):value is string=>typeof value==='string'&&/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(value)&&value!=='00000000-0000-0000-0000-000000000000';
export function findDraftRecovery(actorId:string,appId:string,viewId:string):ApplicationPacket|null{
 if(![actorId,appId,viewId].every(id))return null;
 return scopedUnconfirmed(actorId).find(packet=>{
  const resource=packet.resource;
  if(packet.actorId!==actorId||resource?.kind!=='draft'||resource.appId!==appId||resource.viewId!==viewId||!id(packet.operationId))return false;
  const creating=packet.method==='POST',key=creating?resource.creationNonce:resource.id;
  if(!id(key)||(creating?resource.id!==undefined:resource.creationNonce!==undefined)||packet.scope!==JSON.stringify([`draft-editor:${appId}:${viewId}:${key}`,resource]))return false;
  const base=`applications/${appId}/forms/${viewId}/drafts`,path=creating?base:base+'/'+resource.id;
  if(packet.method==='DELETE'){
   const query=packet.query;
   return packet.expectedStatus===204&&packet.body===undefined&&!!query&&Object.keys(query).length===2&&query.operationId===packet.operationId&&Number.isSafeInteger(query.expectedDraftVersion)&&(query.expectedDraftVersion??0)>0&&packet.path===path+'?operationId='+packet.operationId+'&expectedDraftVersion='+query.expectedDraftVersion;
  }
  return (creating||packet.method==='PATCH')&&packet.query===undefined&&packet.expectedStatus===(creating?201:200)&&packet.path===path&&packet.body?.operationId===packet.operationId;
 })??null;
}
