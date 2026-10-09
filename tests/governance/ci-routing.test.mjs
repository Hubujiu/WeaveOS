import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync,mkdirSync,writeFileSync,rmSync,renameSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join,dirname} from 'node:path';
import {execFileSync} from 'node:child_process';
import {classifyChanges,planForCheckout} from '../../scripts/ci-routing.mjs';
const full={backend:true,browser:true,product:true};
const docs={backend:false,browser:false,product:false};
const backend={backend:true,browser:false,product:false};
const front={backend:false,browser:true,product:true};
const lanes=p=>Object.fromEntries(Object.keys(full).map(k=>[k,p[k]]));
for(const [name,files,expected] of [
 ['prose',['README.md','docs/tasks/V030-059.md'],docs],
 ['compiler',['services/bff/internal/flowgraph/compiler.go'],backend],
 ['codec tests',['services/bff/internal/flowcommands/codec_test.go'],backend],
 ['web',['apps/web/src/App.tsx'],front],
 ['web assets',['apps/web/public/logo.svg','apps/web/src/base.css'],front],
 ['browser tests',['tests/acceptance/web.spec.ts'],front],
 ['mixed',['docs/tasks/V030-059.md','apps/web/src/App.tsx','services/bff/internal/flowgraph/compiler.go'],full],
 ['doc and go',['docs/tasks/V030-059.md','services/bff/internal/flowcommands/codec.go'],backend],
 ['executable docs',['docs/evidence/V010-020/q36-b2/components.config.ts'],full],
 ['instructions',['docs/AGENTS.md'],full],
 ['root instructions',['AGENTS.md'],full],
 ['HTTP',['services/bff/internal/platform/httpserver/server.go'],full],
 ['session',['services/bff/internal/session/session.go'],full],
 ['contracts',['contracts/openapi/openapi.json'],full],
 ['migration',['db/migrations/001.sql'],full],
 ['web configuration',['apps/web/vite.config.ts'],full],
 ['package',['apps/web/package.json'],full],
 ['workflow',['.github/workflows/ci.yml'],full],
 ['router',['scripts/ci-routing.mjs'],full],
 ['unknown',['new/path.xyz'],full],
 ['empty',[],full],
 ['malformed',null,full],
 ['path traversal',['docs/../services/code.md'],full],
 ['linebreak',['docs/new\nfile.md'],full],
 ['many paths',[...Array.from({length:4000},(_,i)=>`docs/prose-${i}.md`),'services/bff/internal/auth/login.go'],full],
])test(`V059 independent routing: ${name}`,()=>assert.deepEqual(lanes(classifyChanges(files)),expected));
function fixture(t){
 const root=mkdtempSync(join(tmpdir(),'weaveos-route-'));t.after(()=>rmSync(root,{recursive:true,force:true}));
 const git=(...args)=>execFileSync('git',args,{cwd:root,encoding:'utf8'}).trim();
 git('init','-q');git('config','user.email','fixture@example.invalid');git('config','user.name','Fixture');
 const put=(p,s)=>{mkdirSync(dirname(join(root,p)),{recursive:true});writeFileSync(join(root,p),s);};
 const commit=()=>{git('add','.');git('commit','-qm','fixture');return git('rev-parse','HEAD');};
 put('README.md','initial\n');put('services/bff/internal/flowgraph/compiler.go','package flowgraph\n');const base=commit();
 return {root,git,put,commit,base};
}
function plan(f,head,base=f.base,extra={}){return planForCheckout({root:f.root,eventName:'pull_request',event:{pull_request:{base:{sha:base,ref:'develop'},head:{sha:head}}},sourceSha:f.git('rev-parse','HEAD'),forceFull:false,...extra});}
test('V059 real Git includes every PR commit, not only last documentation commit',t=>{
 const f=fixture(t);f.put('services/bff/internal/flowgraph/compiler.go','package flowgraph\n// change\n');f.commit();f.put('README.md','updated\n');const head=f.commit();
 const p=plan(f,head);assert.deepEqual(lanes(p),backend);assert.equal(p.sourceSha,head);assert.equal(p.schemaVersion,1);
});
test('V059 real Git uses PR merge base, not unrelated base-side changes',t=>{
 const f=fixture(t);f.git('checkout','-qb','topic');f.put('README.md','topic\n');const head=f.commit();
 f.git('checkout','-qb','base',f.base);f.put('services/bff/internal/auth/base.go','base-only\n');const base=f.commit();f.git('checkout','-q','topic');
 assert.deepEqual(lanes(plan(f,head,base)),docs);
});
test('V059 real Git rename cannot conceal removed backend file behind docs extension',t=>{
 const f=fixture(t);mkdirSync(join(f.root,'docs'));renameSync(join(f.root,'services/bff/internal/flowgraph/compiler.go'),join(f.root,'docs/moved.md'));const head=f.commit();
 assert.deepEqual(lanes(plan(f,head)),backend);
});
test('V059 real Git deletion, spaces and actual merge candidate are handled',t=>{
 const f=fixture(t);f.git('checkout','-qb','topic');f.put('docs/with spaces.md','prose');const head=f.commit();f.git('checkout','-qb','base',f.base);f.put('README.md','base change\n');const base=f.commit();f.git('merge','--no-ff','-m','merge',head);
 const p=plan(f,head,base);assert.deepEqual(lanes(p),docs);assert.equal(p.sourceSha,f.git('rev-parse','HEAD'));
});
test('V059 main target, manual, forced release, missing refs, symlink and unbound candidate run full',t=>{
 const f=fixture(t);f.put('README.md','topic');const head=f.commit();
 for(const extra of [{eventName:'workflow_dispatch'},{forceFull:true},{event:{pull_request:{base:{ref:'main',sha:f.base},head:{sha:head}}}},{event:{pull_request:{base:{ref:'develop',sha:'f'.repeat(40)},head:{sha:head}}}},{sourceSha:'0'.repeat(40)}]) assert.deepEqual(lanes(plan(f,head,f.base,extra)),full);
 f.git('update-index','--chmod=+x','README.md');f.git('commit','-qm','mode');assert.deepEqual(lanes(plan(f,f.git('rev-parse','HEAD'))),full);
});
