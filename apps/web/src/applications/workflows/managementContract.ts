export type WorkflowManagementAction='enable'|'close';
export type WorkflowManagementReceipt={operationId:string;id:string;revision:number;currentVersion:number;candidateVersion:number;state:'enabled'|'disabled'|'closing'};
export function validWorkflowManagementRequest(_resource:unknown,_request:unknown):boolean{return false;}
export function validWorkflowManagementReceipt(_value:unknown,_operationId:string,_flowId:string,_expectedRevision:number,_action:WorkflowManagementAction):_value is WorkflowManagementReceipt{return false;}
