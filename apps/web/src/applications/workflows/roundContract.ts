export type RoundPreview={instanceId:string;flowId:string;roundNumber:number;latestInstanceId:string;state:string;definitionVersion:number;workflowRevision:number;schemaVersion:number;recordVersion:number;canRework:boolean;canResubmit:boolean;canReview:boolean;editableFieldIds:string[]};
export type RoundStartReceipt={operationId:string;flowId:string;previousInstanceId:string;instanceId:string;roundKind:'resubmit'|'review';roundNumber:number;status:'accepted'};
const object=(v:unknown):v is Record<string,unknown>=>!!v&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:string[])=>Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const id=(v:unknown):v is string=>typeof v==='string'&&/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(v)&&v!=='00000000-0000-0000-0000-000000000000';
const positive=(v:unknown)=>typeof v==='number'&&Number.isSafeInteger(v)&&v>0;
export function validRoundPreview(value:unknown,instanceId:string):value is RoundPreview{
 if(!id(instanceId)||!object(value)||!exact(value,['instanceId','flowId','roundNumber','latestInstanceId','state','definitionVersion','workflowRevision','schemaVersion','recordVersion','canRework','canResubmit','canReview','editableFieldIds'])||value.instanceId!==instanceId||!id(value.flowId)||!id(value.latestInstanceId)||!['roundNumber','definitionVersion','workflowRevision','schemaVersion','recordVersion'].every(k=>positive(value[k]))||!['canRework','canResubmit','canReview'].every(k=>typeof value[k]==='boolean')||typeof value.state!=='string'||!['starting','active','completed','rejected','withdrawn','no_effect'].includes(String(value.state))||!Array.isArray(value.editableFieldIds)||!value.editableFieldIds.every(id)||new Set(value.editableFieldIds).size!==value.editableFieldIds.length)return false;
 if(value.latestInstanceId!==instanceId&&(value.canRework||value.canResubmit||value.canReview||value.editableFieldIds.length))return false;
 if((value.canRework||value.canResubmit)&&value.state!=='rejected'&&value.state!=='withdrawn')return false;
 if(value.canReview&&!['completed','rejected','withdrawn'].includes(String(value.state)))return false;
 return true;
}
export function validRoundStartReceipt(value:unknown,operationId:string,flowId:string,previousInstanceId:string,kind:'resubmit'|'review'):value is RoundStartReceipt{
 return [operationId,flowId,previousInstanceId].every(id)&&(kind==='resubmit'||kind==='review')&&object(value)&&exact(value,['operationId','flowId','previousInstanceId','instanceId','roundKind','roundNumber','status'])&&value.operationId===operationId&&value.flowId===flowId&&value.previousInstanceId===previousInstanceId&&id(value.instanceId)&&value.instanceId!==previousInstanceId&&value.roundKind===kind&&positive(value.roundNumber)&&value.status==='accepted';
}

export function validRoundStartRequest(resource:unknown,request:unknown):boolean{
 if(!object(resource)||!exact(resource,['kind','appId','viewId','id','instanceId','flowId','roundKind'])||resource.kind!=='round-start'||!['appId','viewId','id','instanceId','flowId'].every(k=>id(resource[k]))||(resource.roundKind!=='resubmit'&&resource.roundKind!=='review')||!object(request))return false;
 const path=`applications/${resource.appId}/forms/${resource.viewId}/records/${resource.id}/workflows/${resource.instanceId}/round-actions`;
 return request.path===path&&request.method==='POST'&&request.expectedStatus===202&&object(request.body)&&exact(request.body,['kind','expectedWorkflowRevision','expectedSchemaVersion','expectedRecordVersion'])&&request.body.kind===resource.roundKind&&positive(request.body.expectedWorkflowRevision)&&positive(request.body.expectedSchemaVersion)&&positive(request.body.expectedRecordVersion);
}
