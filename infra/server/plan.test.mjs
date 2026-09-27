import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, rmSync, readFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { serverCompose, requireEmptyDirectory } from './plan.mjs';
const runtime=JSON.parse(readFileSync(new URL('../runtime/compose.json',import.meta.url)));
test('Q15 server exposes only loopback HTTPS and preserves internal data network',()=>{
 const c=serverCompose(runtime);
 assert.deepEqual(c.services?.nginx?.ports,['127.0.0.1:19443:19443']);
 assert.equal(c.networks?.default?.internal,true);
 for(const s of Object.values(c.services)) assert.equal(s.build,undefined);
 for(const name of ['postgres','redis','bff','audit-maintenance']) assert.equal(c.services[name].ports,undefined);
});
test('finished services restart automatically and have finite resource and log budgets',()=>{
 const c=serverCompose(runtime);
 assert.deepEqual(Object.keys(c.services??{}).sort(),['audit-maintenance','bff','nginx','postgres','redis']);
 for(const s of Object.values(c.services)){assert.equal(s.restart,'unless-stopped');assert.ok(s.mem_limit);assert.ok(s.logging.options['max-size']);}
 assert.equal(c.services.bff.environment.WEAVEOS_PUBLIC_ORIGIN,'https://localhost:19443');
});
test('deployment refuses every existing directory, including apparently empty ones',()=>{
 const parent=mkdtempSync(join(tmpdir(),'weaveos-server-'));
 try {const existing=join(parent,'private');mkdirSync(existing);assert.throws(()=>requireEmptyDirectory(existing),/existing/i);assert.doesNotThrow(()=>requireEmptyDirectory(join(parent,'new')));}
 finally {rmSync(parent,{recursive:true});}
});
