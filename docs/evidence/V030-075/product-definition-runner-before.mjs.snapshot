import {isAbsolute,resolve} from 'node:path';
const browsers=['chromium','firefox','webkit'];
export function browserCommand({root,output,exchange,runner,uid,gid,head,browser}){
 const paths=[root,output,exchange];
 if(!paths.every(p=>typeof p==='string'&&isAbsolute(p)&&resolve(p)===p&&!/[\s:,\0]/.test(p))||new Set(paths).size!==3||
 !/^weaveos-v041-formal-\d+-\d+-runner$/.test(runner)||!browsers.includes(browser)||!/^[a-f0-9]{40}$/.test(head)||![uid,gid].every(n=>Number.isSafeInteger(n)&&n>=0))throw Error('Invalid isolated browser command');
 return ['run','--rm','--init','--network','container:'+runner,'--user',`${uid}:${gid}`,
  '-v',`${root}:${root}:ro`,'-v',`${exchange}:${exchange}:ro`,'-v',`${output}:${output}:rw`,'-w',root,
  '-e',`WEAVEOS_PRODUCT_BROWSER_FIXTURE=${exchange}/${browser}.request.json`,'-e',`WEAVEOS_PRODUCT_BROWSER_REPORT=${output}/${browser}.json`,
  '-e',`WEAVEOS_PRODUCT_BROWSER_OUTPUT=${output}/${browser}-results`,'-e',`WEAVEOS_PRODUCT_HEAD=${head}`,'-e','HOME=/tmp',
  '--entrypoint','node','mcr.microsoft.com/playwright:v1.63.0-noble@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27',
  'node_modules/@playwright/test/cli.js','test','--config','services/workflow-engine/playwright.product.config.ts',`--project=${browser}`];
}
export async function runBrowserJourneys({start,ready,browser,signal,finish}){
 const results=[];
 for(const name of browsers){
  const handle=await start(name);
  let report;
  try{await ready(handle);report=await browser(handle);}catch(error){
   try{await signal(handle,false);}catch{}
   try{await finish(handle);}catch{}
   throw error;
  }
  await signal(handle,true);
  const backend=await finish(handle);
  results.push({browser:name,report,backend});
 }
 return results;
}
