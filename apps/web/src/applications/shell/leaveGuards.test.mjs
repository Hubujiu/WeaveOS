import assert from 'node:assert/strict';
import test from 'node:test';
import { LeaveGuards } from './leaveGuards.ts';
import { createLeaveController } from '../forms/leaveGuard.ts';

const scope = { kind: 'designer', actorId: 'actor-a', appId: 'app-a', viewId: 'view-a' };

test('an old unsubscribe cannot clear a newer registration for the same scope', () => {
 const guards = new LeaveGuards();
 const old = guards.register(scope, { getStatus: () => 'draft', prepareLeave: () => ({ ok: true }) });
 const newer = guards.register(scope, { getStatus: () => 'unknown', prepareLeave: () => ({ ok: true }) });
 old();
 assert.deepEqual(guards.snapshot('actor-a').map(entry => entry.status), ['unknown']);
 newer();
 assert.deepEqual(guards.snapshot('actor-a'), []);
});

test('a changed status cannot be approved under an old discard decision', () => {
 const guards = new LeaveGuards();
 let status = 'draft';
 const decisions = [];
 guards.register(scope, createLeaveController(
  () => ({ status, fingerprint: status }),
  decision => { decisions.push(decision); },
 ));
 const snapshot = guards.snapshot('actor-a');
 status = 'unknown';
 assert.deepEqual(guards.prepare(snapshot).ok, false);
 assert.deepEqual(decisions, []);
 assert.deepEqual(guards.snapshot('actor-a').map(entry => entry.status), ['unknown']);
 assert.deepEqual(guards.prepare(guards.snapshot('actor-a')).ok, true);
 assert.deepEqual(decisions, ['retain_operation']);
});

test('actor and view scopes stay isolated', () => {
 const guards = new LeaveGuards();
 guards.register(scope, { getStatus: () => 'draft', prepareLeave: () => ({ ok: true }) });
 guards.register({ ...scope, viewId: 'view-b' }, { getStatus: () => 'unknown', prepareLeave: () => ({ ok: true }) });
 guards.register({ ...scope, actorId: 'actor-b' }, { getStatus: () => 'preflight', prepareLeave: () => ({ ok: true }) });
 assert.deepEqual(guards.snapshot('actor-a').map(entry => entry.status), ['draft', 'unknown']);
 assert.deepEqual(guards.snapshot('actor-b').map(entry => entry.status), ['preflight']);
});

test('changed draft content with the same status requires a new confirmation', () => {
 const guards = new LeaveGuards();
 let text = 'first draft';
 let applied = false;
 guards.register(scope, createLeaveController(
  () => ({ status: 'draft', fingerprint: text }),
  () => { applied = true; },
 ));
 const shown = guards.snapshot('actor-a');
 text = 'changed while dialog was open';
 assert.equal(guards.prepare(shown).ok, false);
 assert.equal(applied, false);
});
