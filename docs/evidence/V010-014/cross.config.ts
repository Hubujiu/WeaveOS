import { defineConfig, devices } from '@playwright/test';
import { resolve } from 'node:path';
import base from '../playwright.component.config';

export default defineConfig({
  ...base,
  testDir: '../src',
  testMatch: 'responsive.component.spec.ts',
  outputDir: './results',
  projects: [
    { name: 'firefox', use: { ...devices['Desktop Firefox'] } },
    { name: 'webkit', use: { ...devices['Desktop Safari'] } },
  ],
  webServer: { ...base.webServer, cwd: resolve(import.meta.dirname, '..') },
});
