export type ProductBrowserFixture={version:1;head:string;browser:'chromium'|'firefox'|'webkit';origin:string;actorId:string;outsiderId:string;appId:string;viewId:string;recordId:string;instanceId:string;taskId:string;fieldId:string;session:string;csrf:string;otherSession:string;otherCsrf:string};
export function validateFixture(fixture:unknown,head:unknown,browser:unknown):ProductBrowserFixture;
export function validateReports(input:unknown):{head:string;browsers:3;browserCases:3;backendCases:3};

export type ProductDefinitionFixture=Omit<ProductBrowserFixture,'version'|'instanceId'|'taskId'>&{version:2};
export function validateDefinitionFixture(fixture:unknown,head:unknown,browser:unknown):ProductDefinitionFixture;
export function validateDefinitionReports(input:unknown):{head:string;browsers:3;browserCases:3;backendCases:3};
export function validateCompleteReports(input:unknown):{head:string;browsers:3;browserCases:6;backendCases:6};
