import { test, expect } from "@playwright/test";
import { login } from "./helpers";

test.describe("Presets", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.goto("/presets");
    await page.waitForTimeout(1000);
  });

  test("presets page loads", async ({ page }) => {
    // Should show preset list or new preset button.
    const hasList = (await page.locator("text=New Preset").count()) > 0 ||
                    (await page.locator("table").count()) > 0;
    expect(hasList).toBeTruthy();
  });

  test("preset designer opens", async ({ page }) => {
    const newBtn = page.locator("text=New Preset").first();
    if (await newBtn.isVisible()) {
      await newBtn.click();
      await page.waitForURL(/\/presets\/new/);
      await page.waitForTimeout(500);
      // Should show DB kind selector.
      await expect(page.locator("text=PostgreSQL").first()).toBeVisible();
    }
  });

  test("preset designer has topology controls", async ({ page }) => {
    await page.goto("/presets/new");
    await page.waitForTimeout(1000);
    // Should have machine spec sliders.
    await expect(page.locator("text=CPUs").first()).toBeVisible();
    await expect(page.locator("text=Memory").first()).toBeVisible();
  });
});
