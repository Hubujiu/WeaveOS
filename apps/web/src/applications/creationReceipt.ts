import type { Application } from './types';

const uuid = /^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/i;

// The creation receipt is valid only for the actor who initiated the operation.
// Keep this predicate shared by the production hook wiring and contract tests.
export const validCreatedApplication = (app: Application, actorId: string) => uuid.test(app?.id) && typeof app.name === 'string' && app.name.trim().length > 0 && app.ownerUserId === actorId && app.policyRevision === 1;
