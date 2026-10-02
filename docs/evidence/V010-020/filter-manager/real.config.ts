import {defineConfig} from '@playwright/test';
import {resolve} from 'node:path';
const baseURL=process.env.WEAVEOS_WEB_URL;
if(!baseURL?.startsWith('https://')||!['localhost','127.0.0.1'].includes(new URL(baseURL).hostname))throw Error('BLOCKED: isolated loopback HTTPS required');
if(!process.env.WEAVEOS_ACCEPTANCE_FIXTURES)throw Error('BLOCKED: private synthetic fixtures required');
export default defineConfig({
 testDir:'../../../../tests/acceptance',testMatch:['filter-manager-web.spec.ts','q36-web.spec.ts'],
 fullyParallel:false,workers:1,retries:0,forbidOnly:true,timeout:120000,expect:{timeout:10000},
 reporter:[['list'],['json',{outputFile:resolve('.work/cloud-filter-manager/real-results.json')}]],
 outputDir:'../../../../.work/cloud-filter-manager/real-results',
 use:{baseURL,ignoreHTTPSErrors:true,viewport:{width:1440,height:1000},trace:'off',video:'off',screenshot:'off'},
 projects:[{name:'chromium',use:{browserName:'chromium'}},{name:'firefox',use:{browserName:'firefox'}},{name:'webkit',use:{browserName:'webkit'}}],
});
