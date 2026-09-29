import { existsSync } from 'node:fs';
import {acmePlan} from './acme.mjs';
// Consume the already verified runtime topology; server adds lifecycle only.
export function serverCompose(runtime,{publicTLS=false,publicAccess=false}={}) {
 if(publicAccess&&!publicTLS)throw new Error('Public access requires public TLS');
 const config=structuredClone(runtime);
 for(const [name,service] of Object.entries(config.services)) service.restart=name==='audit-maintenance'?'no':'unless-stopped';
 if(publicTLS)config.services.bff.environment.WEAVEOS_PUBLIC_ORIGIN=acmePlan().origin;
 if(publicAccess){
  config.services.bff.environment.WEAVEOS_PUBLIC_ORIGIN=`https://${acmePlan().domain}`;
  config.services.nginx.ports=['0.0.0.0:443:19443','0.0.0.0:80:80','127.0.0.1:19443:19443'];
  config.services.nginx.volumes=[...config.services.nginx.volumes,'${WEAVEOS_RUNTIME_DIR}/public-nginx.conf:/etc/nginx/nginx.conf:ro'];
 }
 return config;
}
export function requireEmptyDirectory(directory) {
 if(existsSync(directory)) throw new Error('Preserve existing deployment directory');
}
export function serverSchedules({publicTLS=false}={}) {
 const root='/opt/weaveos-v010';
 const program=`/usr/local/bin/node ${root}/infra/server/operations.mjs`;
 const trust=publicTLS?'':`NODE_EXTRA_CA_CERTS=${root}/tls/cert.pem `;
 return `# Managed WeaveOS V010-011 runtime operations only\nPATH=/usr/local/bin:/usr/bin:/bin\n* * * * * root /usr/bin/flock -n ${root}/logs.lock ${program} logs >> ${root}/operations.log 2>&1\n0 * * * * root ${program} audit >> ${root}/operations.log 2>&1\n*/5 * * * * root ${trust}/usr/bin/flock -n ${root}/monitor.lock ${program} monitor >> ${root}/operations.log 2>&1\n15 3 * * * root /usr/bin/flock -n ${root}/backup.lock ${program} backup >> ${root}/operations.log 2>&1\n`+(publicTLS?`33 2,14 * * * root /usr/bin/flock -n ${root}/acme.lock /usr/local/bin/node ${root}/infra/server/acme-run.mjs renew >> ${root}/operations.log 2>&1\n`:'');
}
export function bootstrapCredentials(seed) {
 if(typeof seed?.admin?.password!=='string'||seed.admin.password.length===0)throw new Error('Bootstrap password is missing');
 return {account:'bootstrap-admin',password:seed.admin.password};
}
