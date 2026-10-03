import type {LeaveController,LeaveDecision,LeaveScope,RegisterLeaveGuard} from './leaveGuard';

type Entry={id:number;scope:LeaveScope;controller:LeaveController};
type GuardWindow=Window&{
  __formsGuardStatus?:()=>string|null;
  __formsGuardPrepare?:(decision:LeaveDecision)=>{ok:boolean;status?:string}|null;
  __formsGuardActiveId?:()=>number|null;
  __formsGuardOldUnsubscribe?:()=>void;
};
const active=new Map<string,Entry>();
const oldUnsubscribes:(()=>void)[]=[];
let nextId=0;
const key=(scope:LeaveScope)=>JSON.stringify([scope.kind,scope.actorId,scope.appId,
  scope.kind==='structure'?null:scope.viewId,
  scope.kind==='record'?scope.recordId:scope.kind==='draft'?scope.draftId:null]);
const current=()=>[...active.values()].at(-1)??null;
export const registerHarnessGuard:RegisterLeaveGuard=(scope,controller)=>{
  const entry={id:++nextId,scope,controller},id=key(scope);
  active.set(id,entry);
  const unsubscribe=()=>{if(active.get(id)===entry)active.delete(id);};
  oldUnsubscribes.push(unsubscribe);
  return unsubscribe;
};
export function exposeHarnessGuard(target:Window){
  const controls=target as GuardWindow;
  controls.__formsGuardStatus=()=>current()?.controller.getStatus()??null;
  controls.__formsGuardPrepare=decision=>current()?.controller.prepareLeave(decision)??null;
  controls.__formsGuardActiveId=()=>current()?.id??null;
  controls.__formsGuardOldUnsubscribe=()=>oldUnsubscribes[0]?.();
}
