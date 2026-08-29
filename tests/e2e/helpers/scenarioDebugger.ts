import { expect, type Page } from "@playwright/test";
import { waitForNoMapLoading, waitForTestBridge } from "./state";

export async function jumpToScenario(page: Page, scenarioName: string) {
  await waitForTestBridge(page);
  const scenarioButton = page.getByRole("button", { name: "Map / Scenarios" });
  if (!(await scenarioButton.isVisible())) {
    await page.getByRole("button", { name: "Open game menu" }).click();
  }
  await scenarioButton.click();
  await expect(page.getByText("Scenario Debugger")).toBeVisible({
    timeout: 15_000,
  });

  await page.getByPlaceholder("Filter scenarios").fill(scenarioName);
  const sceneButton = page
    .locator("button")
    .filter({ hasText: scenarioName })
    .first();
  await expect(sceneButton).toBeVisible({ timeout: 15_000 });
  await sceneButton.click();

  await expect(page.getByText("Scenario Debugger")).toBeHidden({
    timeout: 15_000,
  });
  await page.evaluate(() => {
    if (document.activeElement instanceof HTMLElement) {
      document.activeElement.blur();
    }
  });
  await waitForNoMapLoading(page);
}
