import type {FieldValue,HistoryChange,HistoryValueLabel,RecordItem,RuntimeField,RuntimeView,Scope,UUID} from './contracts';

export type RuntimeDisplayField={field:RuntimeField;readable:boolean;editable:boolean;value?:FieldValue};
export function scopeAllows(_scope:Scope,_actorId:UUID,_createdBy:UUID):boolean{return false;}
export function projectRuntimeFields(_view:RuntimeView,_mode:'create'|'read'|'edit',_actorId:UUID,_record?:RecordItem):RuntimeDisplayField[]{return [];}
export function canOfferHistory(_view:RuntimeView,_field:RuntimeField,_actorId:UUID,_record:RecordItem):boolean{return false;}
export function requiresRuntimeReview(_before:RuntimeView,_after:RuntimeView,_dirty:boolean):boolean{return false;}
export function allowedHistoryValueLabels(_change:HistoryChange):Record<UUID,HistoryValueLabel>{return {};}
