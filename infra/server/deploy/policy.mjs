const digest=/^[a-f0-9]{64}$/;
const migration=/^(migrations|archive-migrations)\/[0-9]{5}_[a-z0-9_]+\.sql$/;
export function validateCompatibility(current,release) {
 if(!/^[a-f0-9]{40}$/.test(release.commit)||!Number.isSafeInteger(release.runId)||release.runId<=current.runId||release.backwardCompatible!==true)throw Error('Release identity/compatibility rejected');
 if(!release.migrations||!release.approved)throw Error('Migration declaration missing');
 for(const [path,hash] of Object.entries(current.migrations))if(release.migrations[path]!==hash)throw Error('Published migration changed or removed');
 for(const [path,hash] of Object.entries(release.migrations))if(!migration.test(path)||!digest.test(hash)||release.approved[path]!==hash)throw Error('Migration not approved');
 return true;
}
export function candidateCompose(base,images) {
 const names=['postgres','redis','bff','audit-maintenance','nginx'];
 if(Object.keys(base.services).sort().join()!==[...names].sort().join()||JSON.stringify(base.volumes)!=='{"pg":{}}'||base.networks.default.internal!==true)throw Error('Runtime topology rejected');
 const mounts={postgres:['pg:/var/lib/postgresql'],redis:['${WEAVEOS_RUNTIME_DIR}/redis.conf:/run/secrets/redis.conf:ro'],bff:[], 'audit-maintenance':[],nginx:['${WEAVEOS_RUNTIME_DIR}/tls:/etc/nginx/tls:ro']};
 const env={postgres:'postgres',redis:'redis',bff:'runtime','audit-maintenance':'maintenance'};
 const keys=new Set(['image','env_file','volumes','healthcheck','mem_limit','cpus','logging','command','user','read_only','cap_drop','security_opt','environment','depends_on','networks','ports']);
 for(const name of names){
  const s=base.services[name];
  if(Object.keys(s).some(k=>!keys.has(k))||JSON.stringify(s.volumes??[])!==JSON.stringify(mounts[name])||JSON.stringify(s.env_file??[])!==JSON.stringify(env[name]?['${WEAVEOS_RUNTIME_DIR}/'+env[name]+'.env']:[]))throw Error('Host access rejected');
  if(name!=='nginx'&&(s.ports||s.networks))throw Error('Private service exposure rejected');
 }
 for(const id of Object.values(images))if(!/^sha256:[a-f0-9]{64}$/.test(id))throw Error('Immutable image required');
 const result=structuredClone(base);
 for(const s of Object.values(result.services))s.restart='unless-stopped';
 result.services.bff.image=result.services['audit-maintenance'].image=images.bff;
 result.services.nginx.image=images.web;
 result.services.bff.environment.WEAVEOS_PUBLIC_ORIGIN='https://weave.hubujiu.site';
 result.services.nginx.ports=['0.0.0.0:443:19443','0.0.0.0:80:80','127.0.0.1:19443:19443'];
 result.services.nginx.volumes.push('${WEAVEOS_RUNTIME_DIR}/public-nginx.conf:/etc/nginx/nginx.conf:ro');
 return result;
}
export function publicNginx(source) {
 if(!source.includes('server_name localhost;')||!source.includes('proxy_set_header X-Forwarded-For $remote_addr;')||!source.includes('add_header Cache-Control "no-store" always;'))throw Error('HTTPS boundary missing');
 return source.replace('server_name localhost;','server_name weave.hubujiu.site;').replace('http {','http {\n server { listen 80; server_name weave.hubujiu.site; return 308 https://weave.hubujiu.site$request_uri; }');
}
export async function promote(ops) {
 let phase='validate';
 try {for(phase of ['validate','backup','migrate','activate','health','record'])await ops[phase]();return {status:'deployed'};}
 catch {
  const result={status:'failed',phase};
  if(['activate','health','record'].includes(phase)){
   try{await ops.restore();await ops.healthPrevious();result.rollback='healthy';}catch{result.rollback='failed';}
  }
  return result;
 }
}
