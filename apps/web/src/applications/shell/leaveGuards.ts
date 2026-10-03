// Registration tokens are instance-specific: a stale StrictMode cleanup must
// never remove the replacement controller for the same actor/app/view.
import type { LeaveController, LeaveDecision, LeaveScope, LeaveStatus, RegisterLeaveGuard } from '../forms';
export type { RegisterLeaveGuard } from '../forms';
export type ActiveLeaveGuard = { scope: LeaveScope; status: LeaveStatus; controller: LeaveController };

const scopeKey = (scope: LeaveScope) => {
 const viewId = 'viewId' in scope ? scope.viewId ?? '' : '';
 const key: unknown[] = [scope.kind, scope.actorId, scope.appId, viewId];
 if (scope.kind === 'record') key.push(scope.recordId ? ['record', scope.recordId] : ['clientDraft', scope.clientDraftId]);
 else if (scope.kind === 'draft') key.push(['draft', scope.draftId]);
 return JSON.stringify(key);
};

export class LeaveGuards {
 private active = new Map<string, { scope: LeaveScope; controller: LeaveController }>();

 register: RegisterLeaveGuard = (scope, controller) => {
  const key = scopeKey(scope);
  const entry = { scope, controller };
  this.active.set(key, entry);
  return () => { if (this.active.get(key) === entry) this.active.delete(key); };
 };

 snapshot(actorId: string): ActiveLeaveGuard[] {
  return [...this.active.values()].filter(entry => entry.scope.actorId === actorId)
   .map(entry => ({ ...entry, status: entry.controller.getStatus() }));
 }

 prepare(snapshot: ActiveLeaveGuard[]): { ok: true } | { ok: false; current: ActiveLeaveGuard[] } {
  // The module's first getStatus() records the exact draft fingerprint shown
  // to the user. Calling it again here would overwrite that observation and
  // allow a changed draft with the same status to pass silently.
  if (snapshot.some(entry => this.active.get(scopeKey(entry.scope))?.controller !== entry.controller))
   return { ok: false, current: this.snapshot(snapshot[0]?.scope.actorId ?? '') };
  for (const entry of snapshot) {
   const decision: LeaveDecision = entry.status === 'write_in_flight' || entry.status === 'unknown' ? 'retain_operation' : 'discard';
   if (!entry.controller.prepareLeave(decision).ok)
    return { ok: false, current: this.snapshot(entry.scope.actorId) };
  }
  return { ok: true };
 }
}
