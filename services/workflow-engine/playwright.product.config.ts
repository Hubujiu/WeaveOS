import {defineConfig,devices} from '@playwright/test';
export default defineConfig({
 testDir:'../..',testMatch:'services/workflow-engine/formal-browser.spec.ts',fullyParallel:false,workers:1,retries:0,forbidOnly:true,timeout:120000,expect:{timeout:5000},
 reporter:[['list'],['json',{outputFile:process.env.WEAVEOS_PRODUCT_BROWSER_REPORT}]],
 outputDir:process.env.WEAVEOS_PRODUCT_BROWSER_OUTPUT,
 use:{trace:'off',video:'off',screenshot:'only-on-failure'},
 projects:[{name:'chromium',use:{...devices['Desktop Chrome']}},{name:'firefox',use:{...devices['Desktop Firefox']}},{name:'webkit',use:{...devices['Desktop Safari']}}],
});
