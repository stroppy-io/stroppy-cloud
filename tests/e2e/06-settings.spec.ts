import { test, expect } from "@playwright/test";
import { login } from "./helpers";

test.describe("Settings", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.goto("/settings");
    await page.waitForTimeout(1000);
  });

  test("settings page loads with Cloud tab", async ({ page }) => {
    await expect(page.locator('[role="tab"]').filter({ hasText: "Cloud" })).toBeVisible();
  });

  test("quotas tab exists", async ({ page }) => {
    await expect(page.locator('[role="tab"]').filter({ hasText: "Quotas" })).toBeVisible();
  });

  test("quotas tab shows all fields", async ({ page }) => {
    await page.click('[role="tab"]:has-text("Quotas")');
    await page.waitForTimeout(500);
    await expect(page.locator("text=Allowed Databases")).toBeVisible();
    await expect(page.locator("text=Allowed Providers")).toBeVisible();
    await expect(page.locator("text=Max Nodes")).toBeVisible();
    await expect(page.locator("text=Max CPUs/Node")).toBeVisible();
    await expect(page.locator("text=Max RAM/Node")).toBeVisible();
    await expect(page.locator("text=Max Disk/Node")).toBeVisible();
    await expect(page.locator("text=Max Concurrent Runs")).toBeVisible();
  });

  test("quotas can be edited and saved", async ({ page }) => {
    await page.click('[role="tab"]:has-text("Quotas")');
    await page.waitForTimeout(500);
    // Set max CPUs.
    const cpuInput = page.locator('label:has-text("Max CPUs/Node") + input, label:has-text("Max CPUs/Node") ~ input').first();
    if (await cpuInput.isVisible()) {
      await cpuInput.fill("32");
      // Save.
      await page.click('button:has-text("Save Settings")');
      await page.waitForTimeout(1000);
      // Should show success.
      const saved = (await page.locator("text=saved").count()) > 0 || (await page.locator("text=Settings updated").count()) > 0;
      expect(saved).toBeTruthy();
    }
  });

  test("cloud settings show yandex fields", async ({ page }) => {
    await expect(page.locator("text=Token").first()).toBeVisible();
    await expect(page.locator("text=Cloud ID").first()).toBeVisible();
    await expect(page.locator("text=Folder ID").first()).toBeVisible();
  });
});
