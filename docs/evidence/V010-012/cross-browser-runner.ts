import { defineConfig, devices } from '@playwright/test';
import config from '../apps/web/playwright.component.config';
export default defineConfig({ ...config, testDir: '../apps/web/src', testMatch: 'responsive.component.spec.ts', webServer: { ...config.webServer, cwd: 'D:/Workspace/WeaveOS-worktrees/V010-012/apps/web' }, projects: [{ name: 'firefox', use: {...devices['Desktop Firefox']} }, { name: 'webkit', use: {...devices['Desktop Safari']} }] });
