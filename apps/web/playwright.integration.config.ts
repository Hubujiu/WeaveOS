import { defineConfig, devices } from '@playwright/test';

// Actual HTTPS/BFF/PostgreSQL/Redis must already be running. No API mocks/server substitutes.
const baseURL = process.env.WEAVEOS_WEB_URL;
if (!baseURL?.startsWith('https://')) throw new Error('Real same-origin HTTPS URL required');
export default defineConfig({
  testDir: '../../tests/acceptance', testMatch: 'web.spec.ts',
  workers: 1, retries: 0, forbidOnly: true,
  use: { baseURL, ignoreHTTPSErrors: true }, // Isolated, self-signed local test certificate only.
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    { name: 'firefox', use: { ...devices['Desktop Firefox'] } },
    { name: 'webkit', use: { ...devices['Desktop Safari'] } },
  ],
});
