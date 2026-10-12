import type {Draft,RuntimeView} from './contracts';
export type DraftEditorContext={draft:Draft;appId:string;schemaVersion:number;viewVersion:number;policyRevision:number;recordVersion:number|null;editableFieldIds:string[];canCommit:boolean};
export function validDraftEditorContext(value:unknown,view:RuntimeView,draftId:string):value is DraftEditorContext{return false;}
