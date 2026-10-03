import { defineConfig, devices } from '@playwright/test';
import { resolve } from 'node:path';

const real = process.env.V030012_REAL_API === 'true';
const root = resolve(__dirname, '../../..') + '/';
const port = Number(process.env.V030012_COMPONENT_PORT || 4173);
export default defineConfig({
 testDir: real ? root + 'tests/acceptance' : root + 'apps/web/src',
 testMatch: real ? 'applications-web.spec.ts' : ['applications.component.spec.ts', 'applications.states.component.spec.ts', 'applications.permissions.component.spec.ts', 'applications.form-shell.component.spec.ts'],
 outputDir: root + '.work/v030012/browser-results',
 workers: 1, retries: 0, timeout: 30_000, expect: { timeout: 5_000 },
 use: { baseURL: real ? 'https://localhost:19443' : 'http://127.0.0.1:' + port, ignoreHTTPSErrors: real, trace: 'off' },
 projects: [
  ['chromium', devices['Desktop Chrome']],
  ['firefox', devices['Desktop Firefox']],
  ['webkit', devices['Desktop Safari']],
 ].flatMap(([name, device]) => [
  { name: String(name), use: { ...(device as typeof devices['Desktop Chrome']) } },
  { name: String(name) + '-reduced', use: { ...(device as typeof devices['Desktop Chrome']), reducedMotion: 'reduce' as const } },
 ]),
 webServer: real ? undefined : { command: 'pnpm exec vite --host 127.0.0.1 --port ' + port, cwd: root + 'apps/web', url: 'http://127.0.0.1:' + port, reuseExistingServer: false, timeout: 120_000 },
});
