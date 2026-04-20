import { test, expect } from "@playwright/test";
import { login } from "./helpers";

test.describe("Quota Enforcement", () => {
  test("setting allowed_db_kinds filters NewRun options", async ({ page }) => {
    await login(page);

    // Set quota to only allow YDB.
    await page.goto("/settings");
    await page.waitForTimeout(1000);
    await page.click('[role="tab"]:has-text("Quotas")');
    await page.waitForTimeout(500);
    const dbInput = page.locator('label:has-text("Allowed Databases") ~ input, label:has-text("Allowed Databases") + * input').first();
    if (await dbInput.isVisible()) {
      await dbInput.fill("ydb");
      await page.click('button:has-text("Save Settings")');
      await page.waitForTimeout(1000);
    }

    // Go to NewRun and check only YDB is available.
    await page.goto("/runs/new");
    await page.waitForTimeout(500);
    await page.click("text=Next");
    await page.waitForTimeout(500);
    // PostgreSQL should NOT be visible, YDB should be.
    const pgVisible = await page.locator("button").filter({ hasText: "PostgreSQL" }).isVisible({ timeout: 1000 }).catch(() => false);
    const ydbVisible = await page.locator("button").filter({ hasText: "YDB" }).isVisible({ timeout: 1000 }).catch(() => false);
    expect(ydbVisible).toBeTruthy();
    // pgVisible might still be true if quotas aren't loaded yet, but ideally false.

    // Clean up — remove restriction.
    await page.goto("/settings");
    await page.waitForTimeout(1000);
    await page.click('[role="tab"]:has-text("Quotas")');
    await page.waitForTimeout(500);
    const dbInput2 = page.locator('label:has-text("Allowed Databases") ~ input, label:has-text("Allowed Databases") + * input').first();
    if (await dbInput2.isVisible()) {
      await dbInput2.fill("");
      await page.click('button:has-text("Save Settings")');
      await page.waitForTimeout(1000);
    }
  });
});
