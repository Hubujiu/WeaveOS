import {defineConfig,devices} from '@playwright/test';
const baseURL=process.env.WEAVEOS_WEB_URL;
if(!baseURL?.startsWith('https://')||!['localhost','127.0.0.1'].includes(new URL(baseURL).hostname))throw Error('BLOCKED: actual isolated loopback HTTPS instance required');
if(!process.env.WEAVEOS_ACCEPTANCE_FIXTURES)throw Error('BLOCKED: private synthetic acceptance fixtures required');
export default defineConfig({
 testDir:'../../../../tests/acceptance',testMatch:'q36-web.spec.ts',
 workers:1,retries:0,forbidOnly:true,timeout:120000,expect:{timeout:10000},reporter:[['list']],
 outputDir:'../../../../.work/q36-b2-real-results',
 use:{baseURL,ignoreHTTPSErrors:true,trace:'off',video:'off',screenshot:'off'},
 projects:[{name:'chromium',use:{...devices['Desktop Chrome']}}],
});
