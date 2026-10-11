export type WorkflowOperation={operationId:string;commandId:string;instanceId:string;status:'pending'|'success'|'no_effect';instanceState?:string;sequence?:number;reason?:string};
const object=(v:unknown):v is Record<string,unknown>=>v!==null&&typeof v==='object'&&!Array.isArray(v);
const exact=(v:Record<string,unknown>,keys:readonly string[])=>Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const id=(v:unknown):v is string=>typeof v==='string'&&/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/.test(v)&&v!=='00000000-0000-0000-0000-000000000000';
const reasons=new Set(['instance_missing','instance_exists','deployment_missing','deployment_mismatch','scope_mismatch','terminal_instance','stale_sequence','stale_fence','stale_schema_version','stale_record_version','task_missing','task_inactive','task_epoch_mismatch','actor_mismatch','withdrawal_forbidden','return_target_unvisited','return_target_forbidden','cancelled']);
export function validWorkflowOperation(value:unknown,expected:{operationId:string;instanceId:string;commandId?:string;action?:'withdraw'},status:number,method:'GET'|'POST'):value is WorkflowOperation{
 if(!object(value)||!id(value.operationId)||value.operationId!==expected.operationId||!id(value.instanceId)||value.instanceId!==expected.instanceId||!id(value.commandId)||expected.commandId!==undefined&&value.commandId!==expected.commandId)return false;
 const base=['operationId','commandId','instanceId','status'];
 if(value.status==='pending')return exact(value,base)&&status===(method==='POST'?202:200);
 if(status!==200||!Number.isSafeInteger(value.sequence))return false;
 if(value.status==='success')return exact(value,[...base,'instanceState','sequence'])&&(value.sequence as number)>=1&&(expected.action==='withdraw'?value.instanceState==='withdrawn':['active','completed','rejected'].includes(String(value.instanceState)));
 return value.status==='no_effect'&&exact(value,[...base,'instanceState','reason','sequence'])&&(value.sequence as number)>=0&&value.instanceState==='unchanged'&&typeof value.reason==='string'&&reasons.has(value.reason);
}
