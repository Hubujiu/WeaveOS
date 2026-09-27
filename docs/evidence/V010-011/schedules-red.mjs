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
export function serverSchedules() { return ''; }
