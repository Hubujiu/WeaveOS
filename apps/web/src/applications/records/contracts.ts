// V030-017 consumption types from accepted V015 ADR §§8, 11, 14.
// V013 owns the eventual HTTP/OpenAPI wire; these declarations do not imply
// that a route exists or that a mutation has been confirmed.
export type UUID=string;
export type Version=number;
export type Scope='none'|'own'|'all';
export type FieldValue=string|boolean|string[]|null;
export type Values=Record<UUID,FieldValue>;
export type FieldKind='text'|'multiline'|'number'|'money'|'date'|'datetime'|'single_select'|'multi_select'|'boolean'|'member'|'department';
export type RuntimeField={
 id:UUID;name:string;kind:FieldKind;required:boolean;
 presentation:{helpText:string|null;displayTimeZone:string|null};
 input:{decimal?:{precision:number;scale:number;roundingPlaces:number;roundingMode:'HALF_UP'|'HALF_EVEN'|'TOWARD_ZERO'|'FLOOR'|'CEILING'};timePrecision?:'minute'|'second'|'millisecond';options?:{id:UUID;label:string}[];referenceKind?:'member'|'department'};
 default?:FieldValue;
 access:{read:Scope;create:boolean;edit:Scope;history:Scope};
 query:{operators:('eq'|'neq'|'gt'|'gte'|'lt'|'lte')[];sortable:boolean;quickSearchable:boolean};
};
export type RuntimeLayoutNode={id:UUID;kind:'field';fieldId:UUID;span?:number}|{id:UUID;kind:'system_field';fieldId:string;span?:number}|{id:UUID;kind:'group';title:string;children:RuntimeLayoutNode[];span?:number}|{id:UUID;kind:'divider'};
export type RuntimeView={
 appId:UUID;tableId:UUID;viewId:UUID;schemaVersion:Version;viewVersion:Version;policyRevision:Version;
 fields:RuntimeField[];layout:RuntimeLayoutNode[];
 capabilities:{create:boolean;read:Scope;edit:Scope;history:Scope;search:boolean;draftCreate:boolean;draftEdit:boolean};
};
export type ReferenceDisplay={id:UUID;label:string;deleted:boolean};
export type RecordItem={
 id:UUID;appId:UUID;tableId:UUID;viewId:UUID;createdBy:UUID;createdAt:string;updatedAt:string;
 recordVersion:Version;schemaVersion:Version;values:Values;
 referenceDisplays:Record<UUID,Record<UUID,ReferenceDisplay>>;
};
export type MutationResult={operationId:UUID;id:UUID;recordVersion:Version;schemaVersion:Version;createdAt:string;updatedAt:string};
export type DraftMutationResult={operationId:UUID;id:UUID;draftVersion:Version};
export type DraftSummary={id:UUID;viewId:UUID;tableId:UUID;targetRecordId:UUID|null;schemaVersion:Version;baseRecordVersion:Version|null;draftVersion:Version;createdAt:string;updatedAt:string;hasConflicts:boolean};
export type DraftConflict={fieldId:UUID|null;reason:'FIELD_REMOVED'|'FIELD_PERMISSION_REVOKED'|'SCHEMA_CHANGED'|'BASE_RECORD_CHANGED'};
export type Draft=DraftSummary&{values:Values;conflicts:DraftConflict[]};
export type HistoryValueLabel={label:string|null;deleted:boolean;labelUnavailable:boolean};
export type HistoryChange={fieldId:UUID;fieldKind:string;before:FieldValue;after:FieldValue;fieldLabel:string;fieldDeleted:boolean;valueLabels:Record<UUID,HistoryValueLabel>};
export type HistoryEvent={id:UUID;recordVersionBefore:Version;recordVersionAfter:Version;actorId:UUID;occurredAt:string;origin:'ordinary'|'task_save';changes:HistoryChange[]};

// V012 validates HTTP status/envelope; these validate only the minimal data
// object after that layer has accepted it. Extra keys must not leak values.
const uuid=/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const object=(value:unknown):value is Record<string,unknown>=>typeof value==='object'&&value!==null&&!Array.isArray(value);
const exact=(value:Record<string,unknown>,keys:readonly string[])=>Object.keys(value).length===keys.length&&keys.every(key=>Object.hasOwn(value,key));
const version=(value:unknown):value is Version=>typeof value==='number'&&Number.isSafeInteger(value)&&value>=0;
const timestamp=(value:unknown):value is string=>typeof value==='string'&&/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/.test(value)&&Number.isFinite(Date.parse(value));
export function isMutationResult(value:unknown,operationId:UUID,recordId?:UUID):value is MutationResult{
 return object(value)&&exact(value,['operationId','id','recordVersion','schemaVersion','createdAt','updatedAt'])&&
  value.operationId===operationId&&uuid.test(operationId)&&typeof value.id==='string'&&uuid.test(value.id)&&(!recordId||value.id===recordId)&&
  version(value.recordVersion)&&version(value.schemaVersion)&&timestamp(value.createdAt)&&timestamp(value.updatedAt);
}
export function isDraftMutationResult(value:unknown,operationId:UUID,draftId?:UUID):value is DraftMutationResult{
 return object(value)&&exact(value,['operationId','id','draftVersion'])&&
  value.operationId===operationId&&uuid.test(operationId)&&typeof value.id==='string'&&uuid.test(value.id)&&(!draftId||value.id===draftId)&&version(value.draftVersion);
}
