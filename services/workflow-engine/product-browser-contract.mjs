import {verifyShards} from '../../scripts/verify-shards.mjs';
const browsers=['chromium','firefox','webkit'];
const keys=['version','head','browser','origin','actorId','outsiderId','appId','viewId','recordId','instanceId','taskId','fieldId','session','csrf','otherSession','otherCsrf'];
const ids=['actorId','outsiderId','appId','viewId','recordId','instanceId','taskId','fieldId'];
const uuid=/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/;
const validHead=head=>typeof head==='string'&&/^[0-9a-f]{40}$/.test(head);
const requireValid=(ok)=>{if(!ok)throw Error('Invalid isolated product browser evidence');};
export function validateFixture(fixture,head,browser){
 requireValid(fixture&&typeof fixture==='object'&&!Array.isArray(fixture)&&Object.keys(fixture).length===keys.length&&keys.every(key=>Object.hasOwn(fixture,key)));
 requireValid(validHead(head)&&fixture.head===head&&fixture.version===1&&browsers.includes(browser)&&fixture.browser===browser);
 let origin;try{origin=new URL(fixture.origin);}catch{throw Error('Invalid isolated product browser evidence');}
 requireValid(origin.protocol==='https:'&&origin.hostname==='127.0.0.1'&&origin.port!==''&&origin.origin===fixture.origin);
 requireValid(ids.every(key=>typeof fixture[key]==='string'&&uuid.test(fixture[key])&&fixture[key]!=='00000000-0000-0000-0000-000000000000')&&fixture.actorId!==fixture.outsiderId);
 requireValid(['session','csrf','otherSession','otherCsrf'].every(key=>typeof fixture[key]==='string'&&/^[A-Za-z0-9_-]{1,512}$/.test(fixture[key])));
 return fixture;
}
export function validateReports({head,baseline,reports,backend}){
 requireValid(validHead(head));
 const coverage=verifyShards({baseline,reports,allowedSkips:[]});
 requireValid(coverage.total===3&&coverage.passed===3&&coverage.skipped===0);
 const seen=new Set();
 for(const identity of coverage.identities){
  const [browser,file,titles]=JSON.parse(identity);
  requireValid(browsers.includes(browser)&&!seen.has(browser)&&file==='services/workflow-engine/formal-browser.spec.ts'&&titles.at(-1)==='real task save, approve, retained evidence and outsider isolation');
  seen.add(browser);
 }
 requireValid(Array.isArray(backend)&&backend.length===3);seen.clear();
 const pkg='github.com/Hubujiu/WeaveOS/services/bff/cmd/bff',name='TestRootProductBrowserJourney';
 for(const report of backend){
  requireValid(report&&report.head===head&&browsers.includes(report.browser)&&!seen.has(report.browser)&&Array.isArray(report.events));seen.add(report.browser);
  let run=0,pass=0,complete=0;
  for(const event of report.events){
   requireValid(event&&event.Package===pkg&&event.Action!=='fail'&&event.Action!=='skip'&&(!event.Test||event.Test===name));
   if(event.Test===name&&event.Action==='run')run++;
   if(event.Test===name&&event.Action==='pass')pass++;
   if(!event.Test&&event.Action==='pass')complete++;
  }
  requireValid(run===1&&pass===1&&complete===1);
 }
 return {head,browsers:3,browserCases:coverage.passed,backendCases:backend.length};
}
// P23 second-journey declarations, intentionally unavailable before their RED.
export function validateDefinitionFixture(){throw Error('Definition browser fixture unavailable');}
export function validateDefinitionReports(){throw Error('Definition browser reports unavailable');}
export function validateCompleteReports(){throw Error('Complete browser reports unavailable');}
