import { type Page, expect } from "@playwright/test";

/** Login as admin and wait for the runs page. */
export async function login(page: Page, user = "admin", pass = "admin") {
  await page.goto("/login");
  await page.fill('input[name="username"], input[type="text"]', user);
  await page.fill('input[type="password"]', pass);
  await page.click('button[type="submit"]');
  // Wait for redirect to runs page or tenant selector.
  await page.waitForURL((url) => !url.pathname.includes("/login"), { timeout: 10_000 });
}

/** Ensure we're on the runs list page. */
export async function ensureRunsPage(page: Page) {
  if (!page.url().includes("/runs") && !page.url().endsWith("/")) {
    await page.goto("/");
  }
  await page.waitForSelector("text=New Run", { timeout: 10_000 });
}

/** Wait for text to appear anywhere on the page. */
export async function waitForText(page: Page, text: string, timeout = 10_000) {
  await expect(page.locator(`text=${text}`).first()).toBeVisible({ timeout });
}

/** Get the value of an input by label text. */
export async function getInputValue(page: Page, label: string): Promise<string> {
  const input = page.locator(`label:has-text("${label}") ~ input, label:has-text("${label}") + * input`).first();
  return (await input.inputValue()) || "";
}
