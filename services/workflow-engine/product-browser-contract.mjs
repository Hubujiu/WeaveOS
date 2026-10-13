import {verifyShards} from '../../scripts/verify-shards.mjs';
const browsers=['chromium','firefox','webkit'];
const keys=['version','head','browser','origin','actorId','outsiderId','appId','viewId','recordId','instanceId','taskId','fieldId','session','csrf','otherSession','otherCsrf'];
const ids=['actorId','outsiderId','appId','viewId','recordId','instanceId','taskId','fieldId'];
const uuid=/^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/;
const validHead=head=>typeof head==='string'&&/^[0-9a-f]{40}$/.test(head);
const requireValid=(ok)=>{if(!ok)throw Error('Invalid isolated product browser evidence');};
function checkedFixture(fixture,head,browser,keys,ids,version){
 requireValid(fixture&&typeof fixture==='object'&&!Array.isArray(fixture)&&Object.keys(fixture).length===keys.length&&keys.every(key=>Object.hasOwn(fixture,key)));
 requireValid(validHead(head)&&fixture.head===head&&fixture.version===version&&browsers.includes(browser)&&fixture.browser===browser);
 let origin;try{origin=new URL(fixture.origin);}catch{throw Error('Invalid isolated product browser evidence');}
 requireValid(origin.protocol==='https:'&&origin.hostname==='127.0.0.1'&&origin.port!==''&&origin.origin===fixture.origin);
 requireValid(ids.every(key=>typeof fixture[key]==='string'&&uuid.test(fixture[key])&&fixture[key]!=='00000000-0000-0000-0000-000000000000')&&fixture.actorId!==fixture.outsiderId);
 requireValid(['session','csrf','otherSession','otherCsrf'].every(key=>typeof fixture[key]==='string'&&/^[A-Za-z0-9_-]{1,512}$/.test(fixture[key])));
 return fixture;
}
function checkedReports({head,baseline,reports,backend},fileName,testTitle,name){
 requireValid(validHead(head));
 const coverage=verifyShards({baseline,reports,allowedSkips:[]});
 requireValid(coverage.total===3&&coverage.passed===3&&coverage.skipped===0);
 const seen=new Set();
 for(const identity of coverage.identities){
  const [browser,file,titles]=JSON.parse(identity);
  requireValid(browsers.includes(browser)&&!seen.has(browser)&&file===fileName&&titles.at(-1)===testTitle);
  seen.add(browser);
 }
 requireValid(Array.isArray(backend)&&backend.length===3);seen.clear();
 const pkg='github.com/Hubujiu/WeaveOS/services/bff/cmd/bff';
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
export function validateFixture(fixture,head,browser){return checkedFixture(fixture,head,browser,keys,ids,1);}
export function validateReports(input){return checkedReports(input,'services/workflow-engine/formal-browser.spec.ts','real task save, approve, retained evidence and outsider isolation','TestRootProductBrowserJourney');}
export function validateDefinitionFixture(fixture,head,browser){
 const existingOnly=key=>key!=='instanceId'&&key!=='taskId';
 return checkedFixture(fixture,head,browser,keys.filter(existingOnly),ids.filter(existingOnly),2);
}
export function validateDefinitionReports(input){return checkedReports(input,'services/workflow-engine/formal-definition-browser.spec.ts','real definition creation, publication, manual start and approval','TestRootProductDefinitionJourney');}
export function validateCompleteReports({approval,definition}){
 requireValid(approval&&definition&&approval.head===definition.head);
 const first=validateReports(approval),second=validateDefinitionReports(definition);
 return {head:first.head,browsers:3,browserCases:first.browserCases+second.browserCases,backendCases:first.backendCases+second.backendCases};
}
