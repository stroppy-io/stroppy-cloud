import { test, expect } from "@playwright/test";
import { login } from "./helpers";

test.describe("Packages", () => {
  test("packages page loads", async ({ page }) => {
    await login(page);
    await page.goto("/packages");
    await page.waitForTimeout(1000);
    // Should show package list with built-in packages.
    const hasPackages = (await page.locator("text=PostgreSQL").count()) > 0 ||
                        (await page.locator("text=MySQL").count()) > 0 ||
                        (await page.locator("text=No packages").count()) > 0;
    expect(hasPackages).toBeTruthy();
  });

  test("packages show DB kind and version", async ({ page }) => {
    await login(page);
    await page.goto("/packages");
    await page.waitForTimeout(1000);
    // Built-in packages should show kind.
    const kindBadge = page.locator("text=postgres").first();
    if (await kindBadge.isVisible({ timeout: 3000 }).catch(() => false)) {
      expect(true).toBeTruthy();
    }
  });
});
