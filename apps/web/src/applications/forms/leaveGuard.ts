export type LeaveScope={kind:'structure'|'designer';actorId:string;appId:string;viewId?:string};
export type LeaveStatus='clean'|'draft'|'preflight'|'write_in_flight'|'unknown';
export type LeaveDecision='discard'|'retain_operation';
export type LeaveResult={ok:true}|{ok:false;status:LeaveStatus};
export type LeaveController={getStatus():LeaveStatus;prepareLeave(decision:LeaveDecision):LeaveResult};
export type RegisterLeaveGuard=(scope:LeaveScope,controller:LeaveController)=>(()=>void);
export type LeaveGuardProps={registerLeaveGuard:RegisterLeaveGuard};
