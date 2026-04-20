import { test, expect } from "@playwright/test";
import { login } from "./helpers";

async function findCompletedRun(page: import("@playwright/test").Page): Promise<boolean> {
  await page.goto("/");
  await page.waitForTimeout(1000);
  // Look for a row with "Completed" badge or a run link in a row without "Failed"/"Running".
  const rows = page.locator("table tbody tr");
  const count = await rows.count();
  for (let i = 0; i < count; i++) {
    const row = rows.nth(i);
    const hasFailed = (await row.getByText("Failed").count()) > 0;
    const hasRunning = (await row.getByText("Running").count()) > 0;
    if (!hasFailed && !hasRunning) {
      const link = row.locator("a").first();
      if (await link.isVisible()) {
        await link.click();
        await page.waitForURL(/\/runs\//, { timeout: 5000 });
        await page.waitForTimeout(1000);
        return true;
      }
    }
  }
  return false;
}

test.describe("Share", () => {
  test("share button visible on completed run", async ({ page }) => {
    await login(page);
    if (!(await findCompletedRun(page))) { test.skip(true, "No completed runs"); return; }
    await expect(page.locator("button").filter({ hasText: "Share" })).toBeVisible({ timeout: 5000 });
  });

  test("share creates link and shows URL", async ({ page }) => {
    await login(page);
    if (!(await findCompletedRun(page))) { test.skip(true, "No completed runs"); return; }

    const shareBtn = page.locator("button").filter({ hasText: "Share" });
    if (!(await shareBtn.isVisible({ timeout: 3000 }).catch(() => false))) {
      test.skip(true, "No Share button");
      return;
    }

    await shareBtn.click();
    await page.waitForTimeout(2000);

    // URL should appear on screen.
    const shareLink = page.locator("a[href*='/share/']");
    await expect(shareLink).toBeVisible({ timeout: 5000 });
    const shareHref = await shareLink.getAttribute("href");
    expect(shareHref).toContain("/share/");
    // Should contain a hex token.
    expect(shareHref).toMatch(/\/share\/[a-f0-9]{32}/);
  });

  test("clicking share again copies same URL", async ({ page }) => {
    await login(page);
    if (!(await findCompletedRun(page))) { test.skip(true, "No completed runs"); return; }

    const shareBtn = page.locator("button").filter({ hasText: /Share|Copied/ });
    if (!(await shareBtn.isVisible({ timeout: 3000 }).catch(() => false))) {
      test.skip(true, "No Share button");
      return;
    }

    await shareBtn.click();
    await page.waitForTimeout(2000);
    // Button should show "Copied!" after clicking.
    await expect(page.locator("button").filter({ hasText: "Copied" })).toBeVisible({ timeout: 3000 });
  });

  test("shared page accessible without auth", async ({ page, context }) => {
    await login(page);
    if (!(await findCompletedRun(page))) { test.skip(true, "No completed runs"); return; }

    const shareBtn = page.locator("button").filter({ hasText: /Share|Copied/ });
    if (!(await shareBtn.isVisible({ timeout: 3000 }).catch(() => false))) {
      test.skip(true, "No Share button");
      return;
    }

    // Create share link if not already created.
    await shareBtn.click();
    await page.waitForTimeout(2000);
    const shareLink = page.locator("a[href*='/share/']");
    if (!(await shareLink.isVisible({ timeout: 3000 }).catch(() => false))) {
      test.skip(true, "Share link not created");
      return;
    }
    const shareUrl = await shareLink.getAttribute("href");

    // Open in a clean context — no auth cookies.
    const newContext = await context.browser()!.newContext();
    const newPage = await newContext.newPage();
    await newPage.goto(shareUrl!);
    await newPage.waitForTimeout(2000);

    // Should show shared run page, NOT login redirect.
    await expect(newPage.getByText("Shared Run").first()).toBeVisible({ timeout: 5000 });
    // Should NOT show login form.
    expect(await newPage.locator('input[type="password"]').count()).toBe(0);

    await newContext.close();
  });

  test("shared page shows overview and metrics tabs", async ({ page, context }) => {
    await login(page);
    if (!(await findCompletedRun(page))) { test.skip(true, "No completed runs"); return; }

    const shareBtn = page.locator("button").filter({ hasText: /Share|Copied/ });
    if (!(await shareBtn.isVisible({ timeout: 3000 }).catch(() => false))) {
      test.skip(true, "No Share button");
      return;
    }

    await shareBtn.click();
    await page.waitForTimeout(2000);
    const shareLink = page.locator("a[href*='/share/']");
    if (!(await shareLink.isVisible({ timeout: 3000 }).catch(() => false))) {
      test.skip(true, "No link");
      return;
    }
    const shareUrl = await shareLink.getAttribute("href");

    const newContext = await context.browser()!.newContext();
    const newPage = await newContext.newPage();
    await newPage.goto(shareUrl!);
    await newPage.waitForTimeout(2000);

    // Should have Overview and Metrics tabs.
    await expect(newPage.locator('[role="tab"]').filter({ hasText: "Overview" })).toBeVisible();
    await expect(newPage.locator('[role="tab"]').filter({ hasText: "Metrics" })).toBeVisible();

    await newContext.close();
  });
});
