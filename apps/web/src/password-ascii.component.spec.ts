import { test, expect } from '@playwright/test';

// Expected values come from the Q13-approved PRD and Figma 40:2 annotation.
test('Q13: space is printable but does not satisfy the special class', async ({ page }) => {
  await page.goto('/register');
  await page.getByLabel('密码', { exact: true }).fill('Aa1 ');
  await expect(page.getByRole('progressbar', { name: '密码强度' })).toHaveAttribute('aria-valuenow', '3');
});

test('Q13: registration rejects Unicode despite having all four ASCII classes', async ({ page }) => {
  let requests = 0;
  await page.route('**/api/v1/registrations', route => {
    requests++;
    return route.fulfill({ status: 400, contentType: 'application/json', body: '{"code":"INPUT_INVALID","data":null}' });
  });
  await page.goto('/register?invitationCode=synthetic-prefill-value');
  await page.getByLabel('账号', { exact: true }).fill('ascii-member');
  await page.getByLabel('密码', { exact: true }).fill('Aa1!中');
  await page.getByLabel('确认密码', { exact: true }).fill('Aa1!中');
  await page.getByRole('button', { name: '注册', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('ASCII');
  expect(requests).toBe(0);
});

test('Q13: printable spaces are preserved when four classes are present', async ({ page }) => {
  let submitted: unknown;
  await page.route('**/api/v1/registrations', route => {
    submitted = route.request().postDataJSON();
    return route.fulfill({ status: 201, contentType: 'application/json', body: '{"code":"OK","data":{"id":"00000000-0000-4000-8000-000000000001","account":"ascii-member"}}' });
  });
  await page.goto('/register?invitationCode=synthetic-prefill-value');
  await page.getByLabel('账号', { exact: true }).fill('ascii-member');
  await page.getByLabel('密码', { exact: true }).fill(' Aa1! ');
  await page.getByLabel('确认密码', { exact: true }).fill(' Aa1! ');
  await page.getByRole('button', { name: '注册', exact: true }).click();
  await expect(page).toHaveURL(/\/login$/);
  expect(submitted).toEqual({ account: 'ascii-member', password: ' Aa1! ', invitationCode: 'synthetic-prefill-value' });
});
