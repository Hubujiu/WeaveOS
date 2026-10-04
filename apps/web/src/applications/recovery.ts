// Per-document recovery survives SPA auth-route unmounts, not a page refresh.
// Keys are verified actor IDs; no other account may read a former actor's draft.
export type ApplicationPacket = {
 actorId: string;
 path: string;
 method: 'POST' | 'PUT' | 'PATCH' | 'DELETE';
 expectedStatus: 200 | 201 | 204;
 scope?: string;
 operationId?: string;
 body?: { operationId?: string; [key: string]: unknown };
 query?: { operationId: string; expectedDraftVersion?: number };
 resource?: { kind: 'record' | 'draft'; appId: string; viewId: string; id?: string; creationNonce?: string };
 context?: unknown;
};
type Recovery = { name: string; packet: ApplicationPacket | null; unknown: boolean; draft?: unknown };
const byActor = new Map<string, Recovery>();
const byScope = new Map<string, Recovery>();
const unsentCancels = new Map<string, Set<{ scope?: string; cancel: () => void }>>();
const scopedKey = (actorId: string, scope: string) => JSON.stringify([actorId, scope]);
const recoveryMap = (scope?: string) => scope ? byScope : byActor;
const recoveryKey = (actorId: string, scope?: string) => scope ? scopedKey(actorId, scope) : actorId;

// Only the preflight before a write leaves this document can be cancelled.
// Sent writes remain in byActor until their original operation is confirmed.
export function registerUnsentPreflight(actorId: string, cancel: () => void, scope?: string): () => void {
 let active = unsentCancels.get(actorId);
 if (!active) { active = new Set(); unsentCancels.set(actorId, active); }
 const entry = { scope, cancel };
 active.add(entry);
 return () => {
  active.delete(entry);
  if (!active.size && unsentCancels.get(actorId) === active) unsentCancels.delete(actorId);
 };
}

export function cancelUnsentPreflights(actorId: string, scope?: string) {
 for (const entry of [...(unsentCancels.get(actorId) ?? [])]) if (scope === undefined || entry.scope === scope) entry.cancel();
}

export function getRecovery(actorId: string, scope?: string): Recovery | null { return recoveryMap(scope).get(recoveryKey(actorId, scope)) ?? null; }
export function keepScopedDraft(actorId: string, scope: string, draft: unknown) {
 const key = recoveryKey(actorId, scope), current = byScope.get(key);
 byScope.set(key, { name: current?.name ?? '', packet: current?.packet ?? null, unknown: current?.unknown ?? false, draft });
}
export function clearScopedDraft(actorId: string, scope: string) {
 const key = recoveryKey(actorId, scope), current = byScope.get(key);
 if (current?.packet || current?.unknown) byScope.set(key, { ...current, draft: undefined });
 else byScope.delete(key);
}
export function clearScopedDrafts(actorId: string, scopePrefix: string) {
 for (const [key, value] of byScope) {
  if (key.startsWith('["' + actorId + '","' + scopePrefix)) {
   if (value.packet || value.unknown) byScope.set(key, { ...value, draft: undefined });
   else byScope.delete(key);
  }
 }
}
export function scopedUnconfirmed(actorId: string): ApplicationPacket[] {
 return [...byScope.entries()].filter(([key, value]) => key.startsWith('["' + actorId + '",') && value.unknown && value.packet).map(([, value]) => value.packet!);
}

export function keepName(actorId: string, name: string) {
 const current = byActor.get(actorId);
 byActor.set(actorId, { name, packet: current?.packet ?? null, unknown: current?.unknown ?? false });
}

export function keepPacket(packet: ApplicationPacket, unknown: boolean) {
 const map = recoveryMap(packet.scope), key = recoveryKey(packet.actorId, packet.scope);
 const current = map.get(key);
 const snapshot = deepFreeze(structuredClone(packet));
 map.set(key, {
  name: typeof packet.body?.name === 'string' ? packet.body.name : current?.name ?? '',
  packet: snapshot, unknown, draft: current?.draft,
 });
}

function deepFreeze<T>(value: T): T {
 if (value && typeof value === 'object') {
  for (const child of Object.values(value)) deepFreeze(child);
  Object.freeze(value);
 }
 return value;
}

export function clearPacket(actorId: string, scope?: string) {
 const map = recoveryMap(scope), key = recoveryKey(actorId, scope);
 const current = map.get(key);
 if (current) {
  if (scope && current.draft === undefined) map.delete(key);
  else map.set(key, { ...current, packet: null, unknown: false });
 }
}

export function clearRecovery(actorId: string, scope?: string) { recoveryMap(scope).delete(recoveryKey(actorId, scope)); }
