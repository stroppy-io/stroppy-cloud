import { test, expect } from "@playwright/test";
import { login } from "./helpers";

test.describe("Compare", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test("compare page loads with input fields", async ({ page }) => {
    await page.goto("/compare");
    await page.waitForTimeout(1000);
    await expect(page.locator("text=Compare Runs").first()).toBeVisible();
    await expect(page.locator("text=Run A").first()).toBeVisible();
    await expect(page.locator("text=Run B").first()).toBeVisible();
  });

  test("compare requires two run IDs", async ({ page }) => {
    await page.goto("/compare");
    const compareBtn = page.locator("button").filter({ hasText: "Compare" }).first();
    await expect(compareBtn).toBeDisabled();
  });
});
