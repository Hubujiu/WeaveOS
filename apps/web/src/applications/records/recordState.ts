import type {MutationResult,UUID} from './contracts';

export type NewRecordIdentity={kind:'new';actorId:UUID;appId:UUID;viewId:UUID;clientDraftId:UUID;recordId?:never;draftId?:never};
export type SavedRecordIdentity={kind:'record';actorId:UUID;appId:UUID;viewId:UUID;recordId:UUID;clientDraftId?:never;draftId?:never};
export type SavedDraftIdentity={kind:'draft';actorId:UUID;appId:UUID;viewId:UUID;draftId:UUID;recordId?:never;clientDraftId?:never};
export type RecordEditorIdentity=NewRecordIdentity|SavedRecordIdentity|SavedDraftIdentity;

export function createNewRecordIdentity(_actorId:UUID,_appId:UUID,_viewId:UUID):NewRecordIdentity{return {kind:'new',actorId:'',appId:'',viewId:'',clientDraftId:''};}
export function fieldRendererKey(_identity:RecordEditorIdentity,_fieldId:UUID):string{return '';}
export function confirmRecordIdentity(_identity:NewRecordIdentity,_result:unknown,_operationId:UUID):SavedRecordIdentity|null{return null;}
