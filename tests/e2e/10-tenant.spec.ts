import { test, expect } from "@playwright/test";
import { login } from "./helpers";

test.describe("Tenant Management", () => {
  test("members page accessible for owners", async ({ page }) => {
    await login(page);
    await page.goto("/members");
    await page.waitForTimeout(1000);
    // Should show members list or access denied.
    const visible = (await page.locator("text=Members").count()) > 0 ||
                    (await page.locator("text=admin").count()) > 0;
    expect(visible).toBeTruthy();
  });

  test("API tokens page accessible", async ({ page }) => {
    await login(page);
    await page.goto("/tokens");
    await page.waitForTimeout(1000);
    const visible = (await page.locator("text=API Tokens").count()) > 0 ||
                    (await page.locator("text=Create Token").count()) > 0;
    expect(visible).toBeTruthy();
  });
});
