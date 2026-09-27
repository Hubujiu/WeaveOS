const invalid=()=>new Error('Private credential file rejected');
function checked(id,key) {
 if(![id,key].every(v=>typeof v==='string'&&/^[A-Za-z0-9_-]+$/.test(v)))throw invalid();
 return {Tencent_SecretId:id,Tencent_SecretKey:key};
}
export function credentialsFromCSV(text) {
 // Tencent export uses one row of ASCII credentials. Reject instructions,
 // ambiguous rows and extra fields rather than trying to evaluate anything.
 const rows=text.replace(/^\uFEFF/,'').trim().split(/\r?\n/).map(row=>row.split(',').map(cell=>cell.trim().replace(/^"(.*)"$/,'$1')));
 if(rows.length!==2||rows[0].length!==2||rows[1].length!==2)throw invalid();
 const id=rows[0].indexOf('SecretId'),key=rows[0].indexOf('SecretKey');
 if(id<0||key<0||id===key)throw invalid();
 return checked(rows[1][id],rows[1][key]);
}
export function credentialsFromEnv(text) {
 const result={};
 for(const line of text.trim().split(/\r?\n/)){
  const m=/^(Tencent_SecretId|Tencent_SecretKey)=([A-Za-z0-9_-]+)$/.exec(line);
  if(!m||m[1]in result)throw invalid();result[m[1]]=m[2];
 }
 return checked(result.Tencent_SecretId,result.Tencent_SecretKey);
}
