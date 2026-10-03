import {defineConfig,devices} from '@playwright/test';

// V030-014 component boundary only; the eventual product E2E uses a real BFF/PG.
export default defineConfig({
  testDir: '.',testMatch:['forms.component.spec.ts','forms.renderer.component.spec.ts'],workers:1,retries:0,timeout:30_000,
  expect:{timeout:5_000},use:{baseURL:'http://127.0.0.1:4173'},
  projects:[
    {name:'chromium',use:{...devices['Desktop Chrome']}},
    {name:'firefox',use:{...devices['Desktop Firefox']}},
    {name:'webkit',use:{...devices['Desktop Safari']}},
  ],
  webServer:{command:'pnpm exec vite --host 127.0.0.1 --port 4173',cwd:new URL('../../../',import.meta.url).pathname,
    url:'http://127.0.0.1:4173',
    reuseExistingServer:false,timeout:120_000},
});
