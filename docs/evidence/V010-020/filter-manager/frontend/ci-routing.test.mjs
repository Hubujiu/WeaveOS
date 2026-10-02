import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
test('approved filter-manager browser suite is registered in the existing CI browser job',()=>{
 const source=readFileSync(new URL('../../../../../.github/workflows/ci.yml',import.meta.url),'utf8');
 assert.match(source,/pnpm exec playwright test --config docs\/evidence\/V010-020\/filter-manager\/frontend\/components\.config\.ts/);
});
