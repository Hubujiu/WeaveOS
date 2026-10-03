import { defineConfig, devices } from '@playwright/test';
const baseURL = process.env.WEAVEOS_WEB_URL;
if (!baseURL?.startsWith('https://')) throw new Error('Real HTTPS URL required');
export default defineConfig({testDir: '.',testMatch:'diagnostic-web.spec.ts',workers:1,retries:0,forbidOnly:true,reporter:[['line']],outputDir:'results-diagnostic',use:{baseURL,ignoreHTTPSErrors:true},projects:[{name:'chromium',use:{...devices['Desktop Chrome']}},{name:'firefox',use:{...devices['Desktop Firefox']}},{name:'webkit',use:{...devices['Desktop Safari']}}]});
