import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
test('root-approved CI browser job continuously runs pure filters and Q36 B1/B2 components',()=>{
 const workflow=readFileSync('.github/workflows/ci.yml','utf8');
 const browser=workflow.slice(workflow.indexOf('\n  browser:'));
 const install=browser.indexOf('pnpm exec playwright install --with-deps chromium firefox webkit');
 assert.ok(install>=0);
 const filters=browser.indexOf('node apps/web/src/q36-front-filter.test.mjs');
 const components=browser.indexOf("pnpm exec playwright test --config docs/evidence/V010-020/q36-b2/components.config.ts 'q36-front-|q36-b2\\.component' --output test-results/q36-components");
 assert.ok(filters>install,'pure filters must execute after browser installation');
 assert.ok(components>install,'both Q36 component families must execute using the committed config');
 assert.match(workflow,/permissions:\s*\n\s+contents: read/);
 assert.ok(browser.includes('pnpm exec playwright test tests/e2e'),'existing real BFF smoke retained');
});
