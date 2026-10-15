export type ManualStartOption={flowId:string;name:string;workflowRevision:number;definitionVersion:number;schemaVersion:number;recordVersion:number};
export type ManualStartPage={items:ManualStartOption[];total:number;page:number;pageSize:number;queryVersion:string};
export type ManualStartReceipt={operationId:string;flowId:string;instanceId:string;status:'accepted'|'ignored';ignored:boolean};
export function validManualStartPage(_value:unknown,_page:number,_pageSize:number):_value is ManualStartPage{return false;}
export function validManualStartReceipt(_value:unknown,_operationId:string,_flowId:string):_value is ManualStartReceipt{return false;}
