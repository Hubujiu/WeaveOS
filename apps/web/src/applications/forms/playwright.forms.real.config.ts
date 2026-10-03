import {defineConfig,devices} from '@playwright/test';
export default defineConfig({
  testDir:'.',testMatch:'forms.real.spec.ts',workers:1,retries:0,timeout:60_000,
  expect:{timeout:10_000},use:{baseURL:'http://127.0.0.1:4173'},
  projects:[
    {name:'chromium',use:{...devices['Desktop Chrome']}},
    {name:'firefox',use:{...devices['Desktop Firefox']}},
    {name:'webkit',use:{...devices['Desktop Safari']}},
  ],
  webServer:{command:'pnpm exec vite --host 127.0.0.1 --port 4173',cwd:new URL('../../../',import.meta.url).pathname,
    url:'http://127.0.0.1:4173',reuseExistingServer:false,timeout:120_000},
});
