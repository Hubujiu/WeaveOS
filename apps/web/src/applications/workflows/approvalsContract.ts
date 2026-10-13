import type {WorkflowEvent} from './historyContract';
export type WorkflowApproval={appId:string;viewId:string;recordId:string;event:WorkflowEvent};
export type WorkflowApprovalsPage={items:WorkflowApproval[];total:number;page:number;pageSize:number;queryVersion:string};
export function validWorkflowApprovalsPage(_value:unknown,_page:number,_pageSize:number,_actorId:string):_value is WorkflowApprovalsPage{return false;}
