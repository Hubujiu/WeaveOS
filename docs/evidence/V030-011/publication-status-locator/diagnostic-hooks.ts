// Diagnostic-only transport scheduling. All API responses come from the real BFF.
let releaseEvents: () => void;
let gate: Promise<void>;
let heldEvents = 0;
let saveProofs: Promise<unknown>[] = [];
test.beforeEach(async ({ page, context }) => {
 heldEvents = 0; saveProofs = []; gate = new Promise<void>(resolve => { releaseEvents = resolve; });
 await page.route('**/api/v1/personnel/events/search', async route => {
  heldEvents++; await gate;
  try { await route.continue(); } catch (error) {
   if (!page.isClosed() && !/Target.*closed|already handled|Invalid InterceptionId/.test(String(error))) throw error;
  }
 });
 page.on('response', response => {
  const kind = new URL(response.url()).pathname.match(/^\/api\/v1\/personnel\/(templates|identities)$/)?.[1];
  if (!kind || response.request().method() !== 'POST') return;
  const expectedName = response.request().postDataJSON().name;
  saveProofs.push((async () => {
   expect(response.status()).toBe(201);
   const created = (await response.json()).data;
   expect(created.name).toBe(expectedName);
   const persisted = await context.request.get(response.url() + '/' + created.id);
   expect(persisted.status()).toBe(200);
   const saved = (await persisted.json()).data;
   expect(saved.id).toBe(created.id); expect(saved.name).toBe(expectedName);
   return { kind, createStatus: response.status(), readbackStatus: persisted.status(), id: saved.id, name: saved.name };
  })());
 });
});
test.afterEach(async ({ page }, info) => {
 const statuses = await page.getByRole('status').evaluateAll(nodes => nodes.map(node => ({ tag: node.tagName, class: node.className, text: node.textContent })));
 const savedOperations = await Promise.all(saveProofs);
 console.log('STATUS_LOCATOR_REAL_PROOF ' + JSON.stringify({ project: info.project.name, status: info.status, heldEvents, statuses, savedOperations }));
 expect(heldEvents).toBeGreaterThan(0);
 expect(savedOperations.length).toBe(info.status === 'passed' ? 2 : 1);
 releaseEvents();
 await page.unrouteAll({ behavior: 'wait' });
});
