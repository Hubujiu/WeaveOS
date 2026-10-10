import {chromium} from './WeaveOS-worktrees/V030-067/node_modules/@playwright/test/index.mjs';
import {spawn} from 'node:child_process';
import {mkdir,writeFile} from 'node:fs/promises';
const root='/workspace/scratch/75c2273f6a80/WeaveOS-worktrees/V030-067';
const out=root+'/docs/evidence/V030-067/browser-font-diagnosis';await mkdir(out);
const vite=spawn(process.execPath,['node_modules/vite/bin/vite.js','--host','127.0.0.1','--port','43125','--strictPort'],{cwd:root+'/apps/web',stdio:['ignore','pipe','pipe']});let logs='';vite.stdout.on('data',b=>logs+=b);vite.stderr.on('data',b=>logs+=b);
let browser;let result=[];
try{
 for(let i=0;i<100;i++){try{if((await fetch('http://127.0.0.1:43125')).ok)break;}catch{}await new Promise(r=>setTimeout(r,100));}
 browser=await chromium.launch({executablePath:'/usr/bin/chromium',headless:true,args:['--no-sandbox']});
 for(const mode of ['load','domcontentloaded']){
  const page=await browser.newPage();let font=false;await page.route('https://fonts.googleapis.com/**',()=>{font=true;return new Promise(()=>{});});
  let error=null;const start=Date.now();try{await page.goto('http://127.0.0.1:43125/app/admin',{waitUntil:mode,timeout:4000});}catch(e){error=e.name;}
  result.push({mode,heldExternalFont:font,elapsedMs:Date.now()-start,error,readyState:await page.evaluate(()=>document.readyState)});await page.close();
 }
 if(result[0].error!=='TimeoutError'||result[1].error!==null||!result[0].heldExternalFont)throw Error('diagnostic did not reproduce expected load coupling');
 await writeFile(out+'/result.json',JSON.stringify({browser:await browser.version(),sourceHead:'b61de621a8d8584d4b652e27e214c06526ed16aa',scope:'navigation-only diagnostic, not original business test acceptance',results:result},null,2));console.log(JSON.stringify(result));
}finally{if(browser)await browser.close();vite.kill('SIGTERM');await writeFile(out+'/vite.txt',logs);}
