import {defineConfig} from '@playwright/test';
import {resolve} from 'node:path';
const root=resolve(__dirname,'../../../../../');
export default defineConfig({
 testDir:'../../../../../apps/web/src',testMatch:'filter-manager.component.spec.ts',
 fullyParallel:false,workers:1,retries:0,forbidOnly:true,timeout:20000,
 expect:{timeout:3000},outputDir:'../../../../../.work/cloud-filter-manager/frontend-results',
 reporter:[['list'],['json',{outputFile:resolve(root,'.work/cloud-filter-manager/frontend-results.json')}]],
 use:{baseURL:'http://127.0.0.1:43122',viewport:{width:1440,height:1000},trace:'retain-on-failure'},
 projects:[{name:'chromium',use:{browserName:'chromium'}},{name:'firefox',use:{browserName:'firefox'}},{name:'webkit',use:{browserName:'webkit'}}],
 webServer:{command:'pnpm --dir apps/web exec vite --host 127.0.0.1 --port 43122 --strictPort',cwd:root,url:'http://127.0.0.1:43122',reuseExistingServer:false},
});
