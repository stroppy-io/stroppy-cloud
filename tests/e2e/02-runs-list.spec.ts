import { test, expect } from "@playwright/test";
import { login, ensureRunsPage } from "./helpers";

test.describe("Runs List", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await ensureRunsPage(page);
  });

  test("renders runs table", async ({ page }) => {
    // Table or empty state should be visible.
    const hasTable = (await page.locator("table").count()) > 0;
    const hasEmpty = (await page.locator("text=No runs").count()) > 0;
    expect(hasTable || hasEmpty).toBeTruthy();
  });

  test("New Run button navigates to new run page", async ({ page }) => {
    await page.click("text=New Run");
    await page.waitForURL(/\/runs\/new/);
    await expect(page.locator("text=Where to run")).toBeVisible({ timeout: 5000 });
  });

  test("navigation sidebar has expected links", async ({ page }) => {
    await expect(page.locator("text=Presets").first()).toBeVisible();
    await expect(page.locator("text=Packages").first()).toBeVisible();
    await expect(page.locator("text=Settings").first()).toBeVisible();
  });
});
