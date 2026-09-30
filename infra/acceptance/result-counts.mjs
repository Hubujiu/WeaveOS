// Counts must come from completed test reports, never a source-code constant.
export function countAPIReport(report){
 if(typeof report!=='string')throw Error('Incomplete API report');
 const counts={};for(const field of ['tests','pass','fail','cancelled','skipped','todo']){
  const matches=[...report.matchAll(new RegExp('^# '+field+' (\\d+)\\r?$','gm'))];
  if(matches.length!==1)throw Error('Incomplete API report');
  counts[field]=Number(matches[0][1]);if(!Number.isSafeInteger(counts[field]))throw Error('Invalid API count');
 }
 if(counts.tests<1||counts.tests!==counts.pass||['fail','cancelled','skipped','todo'].some(field=>counts[field]!==0))throw Error('API report is not fully passed');
 return counts.pass;
}
export function countBrowserReport(report){
 const stats=report?.stats;
 if(!stats||['expected','unexpected','flaky','skipped'].some(field=>!Number.isSafeInteger(stats[field])||stats[field]<0)||stats.expected<1||stats.unexpected!==0||stats.flaky!==0||stats.skipped!==0)throw Error('Browser report is not fully passed');
 return stats.expected;
}
