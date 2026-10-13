export type TemplateCounts={directories:number;tables:number;fields:number;forms:number;workflows:number;permissionGroups:number};
export type TemplatePreflight={valid:true;counts:TemplateCounts};
export type TemplateImportReceipt={operationId:string;appId:string;policyRevision:1;structureVersion:1;status:'imported'};
export function validTemplatePreflight(_value:unknown):_value is TemplatePreflight{return false;}
export function validTemplateImportReceipt(_value:unknown,_operationId:string):_value is TemplateImportReceipt{return false;}
