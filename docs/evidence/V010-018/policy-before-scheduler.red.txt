const digest=/^[a-f0-9]{64}$/;
const migration=/^(migrations|archive-migrations)\/[0-9]{5}_[a-z0-9_]+\.sql$/;
export function validateCompatibility(current,release) {
 if(!/^[a-f0-9]{40}$/.test(release.commit)||!Number.isSafeInteger(release.runId)||release.runId<=current.runId||release.backwardCompatible!==true)throw Error('Release identity/compatibility rejected');
 if(!release.migrations||!release.approved)throw Error('Migration declaration missing');
 for(const [path,hash] of Object.entries(current.migrations))if(release.migrations[path]!==hash)throw Error('Published migration changed or removed');
 for(const [path,hash] of Object.entries(release.migrations))if(!migration.test(path)||!digest.test(hash)||release.approved[path]!==hash)throw Error('Migration not approved');
 return true;
}
export function validateRuntimeConfig(base,images) {
 const names=['postgres','redis','bff','audit-maintenance','nginx'];
 if(Object.keys(base).sort().join()!=='networks,services,volumes'||Object.keys(base.services).sort().join()!==[...names].sort().join()||JSON.stringify(base.volumes)!=='{"pg":{}}'||JSON.stringify(base.networks)!=='{"default":{"internal":true},"edge":{}}')throw Error('Runtime topology rejected');
 if(JSON.stringify(base.services.nginx.networks)!=='["default","edge"]'||base.services.bff.environment.WEAVEOS_TRUSTED_PROXY_HOSTS!=='nginx')throw Error('Proxy boundary rejected');
 for(const name of ['bff','audit-maintenance']){
  const s=base.services[name];
  if(s.user!=='65532:65532'||s.read_only!==true||JSON.stringify(s.cap_drop)!=='["ALL"]'||JSON.stringify(s.security_opt)!=='["no-new-privileges:true"]')throw Error('Service confinement rejected');
 }
 const mounts={postgres:['pg:/var/lib/postgresql'],redis:['${WEAVEOS_RUNTIME_DIR}/redis.conf:/run/secrets/redis.conf:ro'],bff:[], 'audit-maintenance':[],nginx:['${WEAVEOS_RUNTIME_DIR}/tls:/etc/nginx/tls:ro']};
 const env={postgres:'postgres',redis:'redis',bff:'runtime','audit-maintenance':'maintenance'};
 const keys=new Set(['image','env_file','volumes','healthcheck','mem_limit','cpus','logging','command','user','read_only','cap_drop','security_opt','environment','depends_on','networks','ports']);
 for(const name of names){
  const s=base.services[name];
  if(Object.keys(s).some(k=>!keys.has(k))||JSON.stringify(s.volumes??[])!==JSON.stringify(mounts[name])||JSON.stringify(s.env_file??[])!==JSON.stringify(env[name]?['${WEAVEOS_RUNTIME_DIR}/'+env[name]+'.env']:[]))throw Error('Host access rejected');
  if(name!=='nginx'&&(s.ports||s.networks))throw Error('Private service exposure rejected');
 }
 for(const id of Object.values(images))if(!/^sha256:[a-f0-9]{64}$/.test(id))throw Error('Immutable image required');
 return true;
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
