import type {Draft,RuntimeView} from './contracts';
import {validDraft} from './draftContract';
export type DraftEditorContext={draft:Draft;appId:string;schemaVersion:number;viewVersion:number;policyRevision:number;recordVersion:number|null;editableFieldIds:string[];canCommit:boolean};
export function validDraftEditorContext(value:unknown,view:RuntimeView,draftId:string):value is DraftEditorContext{
 if(!value||typeof value!=='object'||Array.isArray(value))return false;
 const v=value as Record<string,unknown>,keys=['draft','appId','schemaVersion','viewVersion','policyRevision','recordVersion','editableFieldIds','canCommit'];
 if(Object.keys(v).length!==keys.length||!keys.every(key=>Object.hasOwn(v,key))||v.appId!==view.appId||v.schemaVersion!==view.schemaVersion||v.viewVersion!==view.viewVersion||v.policyRevision!==view.policyRevision||!Array.isArray(v.editableFieldIds)||!v.editableFieldIds.every(id=>typeof id==='string')||new Set(v.editableFieldIds).size!==v.editableFieldIds.length||typeof v.canCommit!=='boolean')return false;
 if(!validDraft(v.draft,view.viewId,view.tableId,draftId,v.editableFieldIds))return false;
 const creating=v.draft.targetRecordId===null;
 if(creating?(!view.capabilities.create||!view.capabilities.draftCreate||v.recordVersion!==null):(!view.capabilities.draftEdit||view.capabilities.edit==='none'||typeof v.recordVersion!=='number'||!Number.isSafeInteger(v.recordVersion)||v.recordVersion<1))return false;
 const fields=new Map(view.fields.map(field=>[field.id,field]));
 if(!v.editableFieldIds.every(id=>{const field=fields.get(id);return !!field&&(creating?field.access.create:field.access.edit!=='none');}))return false;
 const eligible=!v.draft.hasConflicts&&v.draft.schemaVersion===view.schemaVersion&&(creating||v.draft.baseRecordVersion===v.recordVersion);
 return v.canCommit===eligible;
}
export function projectDraftEditorFields(view:RuntimeView,context:DraftEditorContext):import('./runtimeModel').RuntimeDisplayField[]{
 if(!validDraftEditorContext(context,view,context.draft.id))return [];
 const mask=new Set(context.editableFieldIds);
 return view.fields.filter(field=>mask.has(field.id)).map(field=>({field,readable:false,editable:true,...(Object.hasOwn(context.draft.values,field.id)?{value:context.draft.values[field.id]}:{})}));
}
