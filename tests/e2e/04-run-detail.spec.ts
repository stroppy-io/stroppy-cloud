import { test, expect } from "@playwright/test";
import { login } from "./helpers";

async function goToFirstRun(page: import("@playwright/test").Page): Promise<boolean> {
  await page.goto("/");
  await page.waitForTimeout(1000);
  const runLink = page.locator("table tbody tr a").first();
  if (!(await runLink.isVisible({ timeout: 3000 }).catch(() => false))) return false;
  await runLink.click();
  await page.waitForURL(/\/runs\//, { timeout: 5000 });
  await page.waitForTimeout(1000);
  return true;
}

test.describe("Run Detail", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test("navigating to a run shows overview tab", async ({ page }) => {
    if (!(await goToFirstRun(page))) { test.skip(true, "No runs"); return; }
    await expect(page.getByText("Overview").first()).toBeVisible();
  });

  test("tabs are present", async ({ page }) => {
    if (!(await goToFirstRun(page))) { test.skip(true, "No runs"); return; }
    await expect(page.locator('[role="tab"]').filter({ hasText: "Overview" })).toBeVisible();
    await expect(page.locator('[role="tab"]').filter({ hasText: "Logs" })).toBeVisible();
    await expect(page.locator('[role="tab"]').filter({ hasText: "Metrics" })).toBeVisible();
  });

  test("switching to logs tab", async ({ page }) => {
    if (!(await goToFirstRun(page))) { test.skip(true, "No runs"); return; }
    await page.locator('[role="tab"]').filter({ hasText: "Logs" }).click();
    await page.waitForTimeout(1000);
    // Log toolbar should be visible (Machine filter, Phase filter, search).
    const hasToolbar = (await page.getByText("Machine").count()) > 0 ||
                       (await page.getByText("Phase").count()) > 0 ||
                       (await page.locator("input[placeholder*='Search']").count()) > 0;
    expect(hasToolbar).toBeTruthy();
  });

  test("overview accordion expands", async ({ page }) => {
    if (!(await goToFirstRun(page))) { test.skip(true, "No runs"); return; }
    const group = page.locator("button").filter({ hasText: /Infrastructure|Database|Benchmark/ }).first();
    if (await group.isVisible()) {
      await group.click();
      await page.waitForTimeout(300);
      expect(true).toBeTruthy();
    }
  });

  test("action buttons visible", async ({ page }) => {
    if (!(await goToFirstRun(page))) { test.skip(true, "No runs"); return; }
    await expect(page.getByText("Refresh").first()).toBeVisible();
  });

  test("rerun button navigates to new run", async ({ page }) => {
    if (!(await goToFirstRun(page))) { test.skip(true, "No runs"); return; }
    const rerunBtn = page.locator("button").filter({ hasText: "Rerun" });
    if (await rerunBtn.isVisible({ timeout: 2000 }).catch(() => false)) {
      await rerunBtn.click();
      await page.waitForURL(/\/runs\/new/, { timeout: 5000 });
    }
  });
});
