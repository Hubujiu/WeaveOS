import { defineConfig, devices } from '@playwright/test';
import base from '../apps/web/playwright.component.config';
export default defineConfig({...base,timeout:60000,expect:{timeout:1500},testDir:'../apps/web/src',outputDir:'q29-test-results',use:{baseURL:'http://127.0.0.1:4174'},webServer:{...base.webServer as object,command:'pnpm exec vite --host 127.0.0.1 --port 4174 --strictPort',url:'http://127.0.0.1:4174',cwd:'D:/Workspace/WeaveOS-worktrees/V010-020/apps/web'},projects:[{name:'chromium',use:{...devices['Desktop Chrome']}}]});

