export type LeaveScope={kind:'structure'|'designer';actorId:string;appId:string;viewId?:string};
export type LeaveStatus='clean'|'draft'|'preflight'|'write_in_flight'|'unknown';
export type LeaveDecision='discard'|'retain_operation';
export type LeaveResult={ok:true}|{ok:false;status:LeaveStatus};
export type LeaveController={getStatus():LeaveStatus;prepareLeave(decision:LeaveDecision):LeaveResult};
export type RegisterLeaveGuard=(scope:LeaveScope,controller:LeaveController)=>(()=>void);
export type LeaveGuardProps={registerLeaveGuard:RegisterLeaveGuard};

type GuardSnapshot={status:LeaveStatus;fingerprint:string};
export function createLeaveController(read:()=>GuardSnapshot,apply:(decision:LeaveDecision)=>void):LeaveController{
  let observed:GuardSnapshot|null=null;
  return {
    getStatus(){const current=read();observed??=current;return current.status;},
    prepareLeave(decision){
      const current=read();
      const allowed=current.status==='clean'||
        (decision==='discard'&&(current.status==='draft'||current.status==='preflight'))||
        (decision==='retain_operation'&&(current.status==='write_in_flight'||current.status==='unknown'));
      if(!observed||observed.fingerprint!==current.fingerprint||!allowed){
        observed=null;return {ok:false,status:current.status};
      }
      observed=null;apply(decision);return {ok:true};
    },
  };
}
