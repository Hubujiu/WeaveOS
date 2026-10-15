export type RoundPreview={instanceId:string;flowId:string;roundNumber:number;latestInstanceId:string;state:string;definitionVersion:number;workflowRevision:number;schemaVersion:number;recordVersion:number;canRework:boolean;canResubmit:boolean;canReview:boolean;editableFieldIds:string[]};
export type RoundStartReceipt={operationId:string;flowId:string;previousInstanceId:string;instanceId:string;roundKind:'resubmit'|'review';roundNumber:number;status:'accepted'};
export function validRoundPreview(value:unknown,instanceId:string):value is RoundPreview{return false;}
export function validRoundStartReceipt(value:unknown,operationId:string,flowId:string,previousInstanceId:string,kind:'resubmit'|'review'):value is RoundStartReceipt{return false;}
