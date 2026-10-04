import {defineConfig} from '@playwright/test';
import base from './playwright.integration.config';
// Root-authored focused applications acceptance configuration.
// The canonical full product runner and its baseline test selection are unchanged.
export default defineConfig({
 ...base,
 testMatch:'applications-web.spec.ts',
 timeout:120_000,
 use:{...base.use,trace:'off',video:'off',screenshot:'off'},
});
