import {defineConfig,devices} from '@playwright/test';
import {resolve} from 'node:path';

// Run from the repository root. Each command may set its own --output directory.
const port=Number(process.env.WEAVEOS_COMPONENT_PORT||43121);
export default defineConfig({
 testDir:'../../../../apps/web/src',
 testMatch:['personnel*.component.spec.ts','q36-front-*.component.spec.ts','q36-b2.component.spec.ts'],
 workers:1,retries:0,forbidOnly:true,timeout:15000,expect:{timeout:3000},reporter:[['list']],
 outputDir:'../../../../.work/q36-b2-component-results',
 use:{baseURL:`http://127.0.0.1:${port}`,trace:'retain-on-failure',screenshot:'only-on-failure',video:'retain-on-failure'},
 projects:[{name:'chromium',use:{...devices['Desktop Chrome']}},{name:'firefox',use:{...devices['Desktop Firefox']}},{name:'webkit',use:{...devices['Desktop Safari']}}],
 webServer:{command:`node node_modules/vite/bin/vite.js --host 127.0.0.1 --port ${port} --strictPort`,cwd:resolve('apps/web'),url:`http://127.0.0.1:${port}/src/q36-front-fixture.html`,reuseExistingServer:true,timeout:30000},
});
