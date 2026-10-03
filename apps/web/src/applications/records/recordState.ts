import {isMutationResult,type UUID} from './contracts';

export type NewRecordIdentity={kind:'new';actorId:UUID;appId:UUID;viewId:UUID;clientDraftId:UUID;recordId?:never;draftId?:never};
export type SavedRecordIdentity={kind:'record';actorId:UUID;appId:UUID;viewId:UUID;recordId:UUID;clientDraftId?:never;draftId?:never};
export type SavedDraftIdentity={kind:'draft';actorId:UUID;appId:UUID;viewId:UUID;draftId:UUID;recordId?:never;clientDraftId?:never};
export type RecordEditorIdentity=NewRecordIdentity|SavedRecordIdentity|SavedDraftIdentity;

export function createNewRecordIdentity(actorId:UUID,appId:UUID,viewId:UUID):NewRecordIdentity{
 return {kind:'new',actorId,appId,viewId,clientDraftId:crypto.randomUUID()};
}
// The same key is used for the controlled FieldRenderer and its reference
// selector, so a different actor/resource/editor instance cannot inherit old
// candidate items or labels. Refresh within one record is V014's responsibility.
export function fieldRendererKey(identity:RecordEditorIdentity,fieldId:UUID):string{
 const id=identity.kind==='new'?identity.clientDraftId:identity.kind==='record'?identity.recordId:identity.draftId;
 return JSON.stringify([identity.actorId,identity.appId,identity.viewId,identity.kind,id,fieldId]);
}
export function confirmRecordIdentity(identity:NewRecordIdentity,result:unknown,operationId:UUID):SavedRecordIdentity|null{
 if(!isMutationResult(result,operationId))return null;
 return {kind:'record',actorId:identity.actorId,appId:identity.appId,viewId:identity.viewId,recordId:result.id};
}
