export type WorkflowEvent={id:string;instanceId:string;flowId:string;flowName:string;flowNameSource:'captured'|'legacy_last_known';nodeId:string|null;targetNodeId:string|null;actorId:string;action:'start'|'agree'|'reject'|'withdraw'|'return';outcome:'success'|'no_effect';sequence:number;schemaVersion:number;recordVersion:number;occurredAt:string};
export type HistoricalField={fieldId:string;fieldName:string;fieldKind:string;value:string|boolean|string[]|null;valueLabels:Record<string,{label:string;deleted:boolean}>};
export type WorkflowEventDetail={event:WorkflowEvent;basis:{status:'available'|'unavailable';fields:HistoricalField[]}};
export function validWorkflowHistory(data:unknown,pagination:unknown,pageSize:number):data is {items:WorkflowEvent[]}{return false;}
export function validWorkflowEventDetail(data:unknown,expected:WorkflowEvent):data is WorkflowEventDetail{return false;}
