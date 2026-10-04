// Screenshot evidence only. Reuses Root's unmodified fixtures; no new test or oracle.
import {readFileSync,mkdirSync,writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
import ts from 'typescript';
import {chromium} from '@playwright/test';
const root=process.cwd(), out=resolve('docs/evidence/V030-021/p1/screenshots');mkdirSync(out,{recursive:true});
const source=p=>readFileSync(resolve('apps/web/src',p),'utf8');
const moduleURL=code=>'data:text/javascript;base64,'+Buffer.from(ts.transpileModule(code,{compilerOptions:{target:ts.ScriptTarget.ES2022,module:ts.ModuleKind.ESNext}}).outputText).toString('base64');
const recordSource=source('root-monochrome-shell.component.spec.ts').split('// Root oracle:')[0].replace(/^import .*;\n/,'');
const record=await import(moduleURL(recordSource+'\nexport {fixture,routePath};'));
const personnelSource=source('personnel.component.spec.ts').split('async function admin(')[0].replace(/^import .*;\n/gm,'');
const personnel=await import(moduleURL(source('personnel-query-fixtures.ts')+'\n'+personnelSource+'\nexport {fixture};'));
const browser=await chromium.launch();const measurements=[];
for(const [width,height] of [[1440,1000],[1280,800],[390,844]]){
 for(const name of ['home','catalogue','records','admin']){
  const page=await browser.newPage({viewport:{width,height},reducedMotion:width===390?'reduce':'no-preference'});
  if(name==='admin')await personnel.fixture(page);else{const api=await record.fixture(page);api.seed('18.00');}
  await page.goto('http://127.0.0.1:4174'+(name==='home'?'/app':name==='catalogue'?'/app/applications':name==='admin'?'/app/admin':record.routePath));
  await page.getByTestId('workspace-shell').waitFor();
  await page.locator(name==='admin'?'.personnel-source-table':name==='records'?'.record-workspace':'.app-card-grid').waitFor();
  await page.evaluate(()=>document.fonts.ready);
  await page.evaluate(()=>Promise.all(document.getAnimations().filter(animation=>animation.effect?.getTiming().iterations!==Infinity).map(animation=>animation.finished.catch(()=>{}))));
  await page.screenshot({path:resolve(out,`${name}-${width}.png`),fullPage:true});
  measurements.push({name,width,height,...await page.evaluate(()=>({scrollWidth:document.documentElement.scrollWidth,viewport:innerWidth,images:[...document.images].map(n=>({src:n.getAttribute('src'),loaded:n.complete&&n.naturalWidth>0,width:n.getBoundingClientRect().width,height:n.getBoundingClientRect().height}))}))});
  await page.close();
 }
}
await browser.close();writeFileSync(resolve(out,'geometry.json'),JSON.stringify(measurements,null,2)+'\n');
