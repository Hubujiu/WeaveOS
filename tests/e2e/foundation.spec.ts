import { test, expect } from '@playwright/test';

test('FND-04: production build mounts without JavaScript errors', async ({ page }) => {
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.goto('/');
  await expect(page).toHaveTitle('WeaveOS');
  await expect(page.locator('#root')).toBeAttached();
  expect(errors).toEqual([]);
});
test('FND-01: browser reaches real Go BFF through the same-origin proxy', async ({ page }) => {
  await page.goto('/');
  const result = await page.evaluate(async () => {
    const response = await fetch('/health/live');
    return { status: response.status, body: await response.json(), id: response.headers.get('X-Request-Id') };
  });
  expect(result.status).toBe(200);
  expect(result.body).toEqual({ status: 'alive' });
  expect(result.id).toMatch(/^[0-9a-f]{32}$/);
});
test('FND-01: unwired product readiness is not a false success', async ({ request }) => {
  const response = await request.get('/health/ready');
  expect(response.status()).toBe(503);
  expect(await response.json()).toEqual({ status: 'not_ready' });
});
test('FND-01: missing APIs never fall back to the SPA', async ({ request }) => {
  const response = await request.get('/api/v1/unknown');
  expect(response.status()).toBe(404);
  expect(response.headers()['content-type']).toContain('application/json');
  expect((await response.json()).code).toBe('API_NOT_FOUND');
});
