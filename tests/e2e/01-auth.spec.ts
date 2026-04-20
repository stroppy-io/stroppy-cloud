import { test, expect } from "@playwright/test";
import { login } from "./helpers";

test.describe("Authentication", () => {
  test("login page renders", async ({ page }) => {
    await page.goto("/login");
    await expect(page.locator('input[type="password"]')).toBeVisible();
    await expect(page.locator('button[type="submit"]')).toBeVisible();
  });

  test("login with valid credentials", async ({ page }) => {
    await login(page);
    // Should land on runs page or tenant selector.
    const url = page.url();
    expect(url.includes("/login")).toBeFalsy();
  });

  test("login with bad credentials shows error", async ({ page }) => {
    await page.goto("/login");
    await page.fill('input[name="username"], input[type="text"]', "admin");
    await page.fill('input[type="password"]', "wrongpassword");
    await page.click('button[type="submit"]');
    // Should stay on login or show error.
    await page.waitForTimeout(2000);
    const url = page.url();
    expect(url.includes("/login") || (await page.locator("text=invalid").count()) > 0 || (await page.locator("text=error").count()) > 0).toBeTruthy();
  });

  test("unauthenticated access redirects to login", async ({ page }) => {
    await page.goto("/runs/new");
    await page.waitForURL(/\/login/, { timeout: 5000 });
  });

  test("refresh maintains session", async ({ page }) => {
    await login(page);
    await page.reload();
    await page.waitForTimeout(1000);
    expect(page.url().includes("/login")).toBeFalsy();
  });
});
