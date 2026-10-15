import type {Draft,DraftSummary} from './contracts';
export function validDraftSummary(value:unknown,viewId:string,tableId:string):value is DraftSummary{return false;}
export function validDraft(value:unknown,viewId:string,tableId:string,draftId:string,fieldIds:readonly string[]):value is Draft{return false;}
