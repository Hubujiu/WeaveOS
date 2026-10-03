// Per-document recovery survives SPA auth-route unmounts, not a page refresh.
// Keys are verified actor IDs; no other account may read a former actor's draft.
export type ApplicationPacket = { actorId: string; path: string; body: { operationId: string; [key: string]: unknown } };
type Recovery = { name: string; packet: ApplicationPacket | null; unknown: boolean };
const byActor = new Map<string, Recovery>();

export function getRecovery(actorId: string): Recovery | null { return byActor.get(actorId) ?? null; }

export function keepName(actorId: string, name: string) {
 const current = byActor.get(actorId);
 byActor.set(actorId, { name, packet: current?.packet ?? null, unknown: current?.unknown ?? false });
}

export function keepPacket(packet: ApplicationPacket, unknown: boolean) {
 const current = byActor.get(packet.actorId);
 byActor.set(packet.actorId, {
  name: typeof packet.body.name === 'string' ? packet.body.name : current?.name ?? '',
  packet, unknown,
 });
}

export function clearPacket(actorId: string) {
 const current = byActor.get(actorId);
 if (current) byActor.set(actorId, { ...current, packet: null, unknown: false });
}

export function clearRecovery(actorId: string) { byActor.delete(actorId); }
