import { test, expect } from "@playwright/test";
import { login } from "./helpers";

test.describe("New Run Wizard", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.goto("/runs/new");
    await page.waitForSelector("text=Where to run", { timeout: 5000 });
  });

  test("step 1: provider selection", async ({ page }) => {
    // Docker and Yandex Cloud should be visible.
    await expect(page.getByText("Docker", { exact: false }).first()).toBeVisible();
    await expect(page.getByText("Yandex Cloud", { exact: false }).first()).toBeVisible();
    // Select Yandex Cloud.
    await page.getByText("Yandex Cloud", { exact: false }).first().click();
    // Platform selector should appear.
    await expect(page.getByText("Platform").first()).toBeVisible();
    await expect(page.getByText("Standard v3").first()).toBeVisible();
  });

  test("step 1: platform selector appears for yandex", async ({ page }) => {
    await page.getByText("Yandex Cloud", { exact: false }).first().click();
    await expect(page.getByText("Standard v2").first()).toBeVisible();
    await expect(page.getByText("Standard v3").first()).toBeVisible();
    await expect(page.getByText("High-freq v3").first()).toBeVisible();
  });

  test("step 2: database selection", async ({ page }) => {
    // Navigate to step 2.
    await page.click("text=Next");
    await page.waitForTimeout(500);
    // DB kind buttons should be visible.
    await expect(page.locator("text=PostgreSQL").first()).toBeVisible();
    await expect(page.locator("text=YDB").first()).toBeVisible();
  });

  test("step 2: version selector is editable combo", async ({ page }) => {
    await page.click("text=Next");
    await page.waitForTimeout(500);
    // Version input should exist and be editable.
    const versionInput = page.locator('input[list^="versions-"]').first();
    await expect(versionInput).toBeVisible();
    // Should have a datalist with suggestions.
    const value = await versionInput.inputValue();
    expect(value).toBeTruthy();
  });

  test("step 2: preset selection", async ({ page }) => {
    await page.click("text=Next");
    await page.waitForTimeout(500);
    // Presets should be loaded.
    const presetButtons = page.locator("[class*=border]").filter({ hasText: /single|cluster|scale/i });
    expect(await presetButtons.count()).toBeGreaterThan(0);
  });

  test("step 3: workload parameters", async ({ page }) => {
    // Skip to step 3.
    await page.click("text=Next");
    await page.waitForTimeout(300);
    await page.click("text=Next");
    await page.waitForTimeout(500);
    // Workload controls should be visible.
    await expect(page.locator("text=Duration").first()).toBeVisible();
    await expect(page.locator("text=VUs").first()).toBeVisible();
    await expect(page.locator("text=Scale Factor").first()).toBeVisible();
  });

  test("step 3: database machine sliders for yandex", async ({ page }) => {
    // Select Yandex first.
    await page.click("text=Yandex Cloud");
    await page.waitForTimeout(300);
    // Go to step 3.
    await page.click("text=Next");
    await page.waitForTimeout(300);
    await page.click("text=Next");
    await page.waitForTimeout(500);
    // Database Machine section should appear.
    await expect(page.locator("text=Database Machine")).toBeVisible();
  });

  test("step 4: review shows execution plan", async ({ page }) => {
    // Navigate through all steps.
    for (let i = 0; i < 3; i++) {
      await page.click("text=Next");
      await page.waitForTimeout(500);
    }
    // Review page should show.
    await expect(page.locator("text=Review & Launch").first()).toBeVisible();
    // Launch button should be visible.
    await expect(page.locator("text=Launch Run")).toBeVisible();
  });

  test("step 4: accordion groups are expandable", async ({ page }) => {
    for (let i = 0; i < 3; i++) {
      await page.click("text=Next");
      await page.waitForTimeout(500);
    }
    await page.waitForTimeout(1000);
    // Click on a group to expand.
    const infraGroup = page.locator("button").filter({ hasText: "Infrastructure" }).first();
    if (await infraGroup.isVisible()) {
      await infraGroup.click();
      await page.waitForTimeout(300);
      // Should show phases or config inside.
      await expect(page.locator("text=Machines").first()).toBeVisible({ timeout: 3000 });
    }
  });

  test("summary sidebar shows config", async ({ page }) => {
    // The right sidebar should show provider, database, etc.
    await expect(page.locator("text=Provider").first()).toBeVisible();
    await expect(page.locator("text=Database").first()).toBeVisible();
  });
});
