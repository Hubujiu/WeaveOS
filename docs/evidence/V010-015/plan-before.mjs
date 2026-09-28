import { existsSync } from 'node:fs';
import {acmePlan} from './acme.mjs';
// Consume the already verified runtime topology; server adds lifecycle only.
export function serverCompose(runtime,{publicTLS=false}={}) {
 const config=structuredClone(runtime);
 for(const service of Object.values(config.services)) service.restart='unless-stopped';
 if(publicTLS)config.services.bff.environment.WEAVEOS_PUBLIC_ORIGIN=acmePlan().origin;
 return config;
}
export function requireEmptyDirectory(directory) {
 if(existsSync(directory)) throw new Error('Preserve existing deployment directory');
}
export function serverSchedules({publicTLS=false}={}) {
 const root='/opt/weaveos-v010';
 const program=`/usr/local/bin/node ${root}/infra/server/operations.mjs`;
 const trust=publicTLS?'':`NODE_EXTRA_CA_CERTS=${root}/tls/cert.pem `;
 return `# Managed WeaveOS V010-011 runtime operations only\nPATH=/usr/local/bin:/usr/bin:/bin\n*/5 * * * * root ${trust}/usr/bin/flock -n ${root}/monitor.lock ${program} monitor >> ${root}/operations.log 2>&1\n15 3 * * * root /usr/bin/flock -n ${root}/backup.lock ${program} backup >> ${root}/operations.log 2>&1\n`+(publicTLS?`33 2,14 * * * root /usr/bin/flock -n ${root}/acme.lock /usr/local/bin/node ${root}/infra/server/acme-run.mjs renew >> ${root}/operations.log 2>&1\n`:'');
}
export function bootstrapCredentials(seed) {
 if(typeof seed?.admin?.password!=='string'||seed.admin.password.length===0)throw new Error('Bootstrap password is missing');
 return {account:'bootstrap-admin',password:seed.admin.password};
}
