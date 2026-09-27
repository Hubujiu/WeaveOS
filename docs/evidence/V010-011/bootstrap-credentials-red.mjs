import { existsSync } from 'node:fs';
// Consume the already verified runtime topology; server adds lifecycle only.
export function serverCompose(runtime) {
 const config=structuredClone(runtime);
 for(const service of Object.values(config.services)) service.restart='unless-stopped';
 return config;
}
export function requireEmptyDirectory(directory) {
 if(existsSync(directory)) throw new Error('Preserve existing deployment directory');
}
export function serverSchedules() {
 const root='/opt/weaveos-v010';
 const program=`/usr/local/bin/node ${root}/infra/server/operations.mjs`;
 return `# Managed WeaveOS V010-011 runtime operations only\nPATH=/usr/local/bin:/usr/bin:/bin\n*/5 * * * * root NODE_EXTRA_CA_CERTS=${root}/tls/cert.pem /usr/bin/flock -n ${root}/monitor.lock ${program} monitor >> ${root}/operations.log 2>&1\n15 3 * * * root /usr/bin/flock -n ${root}/backup.lock ${program} backup >> ${root}/operations.log 2>&1\n`;
}
export function bootstrapCredentials() { return {account:'bootstrap-admin'}; }
