export type RecordWorkflow={id:string;flowId:string;name:string;initiatorId:string;state:'starting'|'active'|'completed'|'rejected'|'withdrawn'|'no_effect';createdAt:string;updatedAt:string;definitionVersion:number;sequence:number};
export type RecordWorkflowPage={items:RecordWorkflow[];total:number;page:number;pageSize:number;queryVersion:string};
export function validRecordWorkflows(_value:unknown,_page:number,_size:number):_value is RecordWorkflowPage{return false;}
