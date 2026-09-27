import { test, expect } from '@playwright/test';

// Oracle: WaveOS Figma Login 13:2 / Register 40:2, 12 columns,
// Updated 2026-09-27: no top/side page padding, 22px bottom padding,
// 32px gutters, card in columns 5–8; full-width top bar with bottom-only corners.
for (const route of ['/login', '/register']) {
  for (const sample of [
    { width: 1920, height: 1080, cardWidth: 618.667 },
    { width: 2560, height: 1440, cardWidth: 832 },
    { width: 1440, height: 900, cardWidth: 458.667 },
  ]) {
    test(`Figma responsive ${route} at ${sample.width}×${sample.height}`, async ({ page }) => {
      await page.setViewportSize(sample);
      await page.goto(route);
      const site = await page.locator('.site').boundingBox();
      const header = await page.locator('.brand-header').boundingBox();
      const card = await page.locator('.auth-card').boundingBox();
      expect(site?.width).toBe(sample.width);
      expect(site?.height).toBeGreaterThanOrEqual(sample.height);
      expect.soft(header?.x).toBe(0);
      expect.soft(header?.y).toBe(0);
      expect.soft(header?.width).toBe(sample.width);
      expect(header?.height).toBe(94);
      const corners = await page.locator('.brand-header').evaluate(element => {
        const style = getComputedStyle(element);
        return [style.borderTopLeftRadius, style.borderTopRightRadius, style.borderBottomRightRadius, style.borderBottomLeftRadius];
      });
      expect.soft(corners).toEqual(['0px', '0px', '22px', '22px']);
      expect(card).not.toBeNull();
      expect.soft(card!.width).toBeCloseTo(sample.cardWidth, 0);
      expect(card!.x + card!.width / 2).toBeCloseTo(sample.width / 2, 0);
      expect.soft(card!.y + card!.height / 2).toBeCloseTo((112 + sample.height - 22) / 2, 0);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      for (const input of await page.locator('.field-input').all()) {
        expect((await input.boundingBox())!.height).toBe(56);
      }
    });
  }
  for (const sample of [{ width: 320, height: 568 }, { width: 390, height: 844 }, { width: 768, height: 600 }]) {
    test(`responsive ${route} stays usable at ${sample.width}×${sample.height}`, async ({ page }) => {
      await page.setViewportSize(sample);
      await page.goto(route);
      const header = (await page.locator('.brand-header').boundingBox())!;
      expect.soft(header.x).toBe(0);
      expect.soft(header.y).toBe(0);
      expect.soft(header.width).toBe(sample.width);
      await expect.soft(page.locator('.brand-header')).toHaveCSS('border-top-left-radius', '0px');
      const card = (await page.locator('.auth-card').boundingBox())!;
      expect(card.x).toBeGreaterThanOrEqual(0);
      expect(card.x + card.width).toBeLessThanOrEqual(sample.width);
      expect(card.x + card.width / 2).toBeCloseTo(sample.width / 2, 0);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      const action = page.getByRole('button', { name: route === '/login' ? '登录' : '注册', exact: true });
      await action.scrollIntoViewIfNeeded();
      const box = (await action.boundingBox())!;
      expect(box.y).toBeGreaterThanOrEqual(0);
      expect(box.y + box.height).toBeLessThanOrEqual(sample.height);
    });
  }
}
