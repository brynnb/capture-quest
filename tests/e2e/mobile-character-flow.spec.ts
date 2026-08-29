import { randomUUID } from "node:crypto";
import { devices, expect, test, type Page } from "@playwright/test";
import { loginAsGuest, uniqueTrainerName } from "./helpers/auth";

test.use({ ...devices["Pixel 5"] });

async function expectMobileViewportFits(page: Page) {
  await expect
    .poll(() =>
      page.evaluate(() => ({
        horizontal: document.documentElement.scrollWidth - window.innerWidth,
        mainVertical:
          (document.querySelector("#main")?.scrollHeight ?? 0) -
          (document.querySelector("#main")?.clientHeight ?? 0),
        creatorVertical: (() => {
          const creator = document.querySelector(
            '[data-testid="character-creator-step"]',
          );
          return creator ? creator.scrollHeight - creator.clientHeight : 0;
        })(),
      })),
    )
    .toEqual({ horizontal: 0, mainVertical: 0, creatorVertical: 0 });
}

async function chooseCurrentMobileOption(page: Page, title: string) {
  const selector = page.getByRole("region", { name: title });
  const option = selector.getByRole("button").nth(1);
  await option.click();
  await expect(option).toHaveAttribute("aria-pressed", "true");
}

async function expectChoiceListReachesNavigation(
  page: Page,
  navigationLabel: string,
) {
  const listBox = await page.getByTestId("compact-choice-list").boundingBox();
  const navigationBox = await page
    .getByRole("button", { name: navigationLabel, exact: true })
    .boundingBox();

  expect(listBox).not.toBeNull();
  expect(navigationBox).not.toBeNull();
  const gap = navigationBox!.y - (listBox!.y + listBox!.height);
  expect(gap).toBeGreaterThanOrEqual(0);
  expect(gap).toBeLessThanOrEqual(32);
  expect(listBox!.height).toBeGreaterThan(120);
}

async function expectMobileNavigationFits(page: Page) {
  const result = await page
    .getByTestId("character-creation-navigation")
    .evaluate((navigation) => {
      const buttons = [...navigation.querySelectorAll("button")];
      return {
        navigation: navigation.getBoundingClientRect().toJSON(),
        buttons: buttons.map((button) => ({
          bounds: button.getBoundingClientRect().toJSON(),
          textFits: button.scrollWidth <= button.clientWidth,
        })),
      };
    });

  expect(result.buttons).toHaveLength(2);
  for (const button of result.buttons) {
    expect(button.bounds.left).toBeGreaterThanOrEqual(0);
    expect(button.bounds.right).toBeLessThanOrEqual(result.navigation.right);
    expect(button.textFits).toBe(true);
  }
  expect(
    Math.abs(result.buttons[0].bounds.width - result.buttons[1].bounds.width),
  ).toBeLessThanOrEqual(1);
}

async function createMobileCharacter(page: Page, name: string) {
  await page
    .getByRole("button", { name: /Create New(?: Character)?/i })
    .first()
    .click();

  await expect(page.getByText("Step 1 of 4 · Trainer")).toBeVisible();
  await expect(page.getByText(/Tap to choose|Selected/i)).toHaveCount(0);
  await expectMobileViewportFits(page);
  await expectMobileNavigationFits(page);
  await page.getByPlaceholder("Enter character name").fill(name);
  await page.getByPlaceholder("Enter rival name").fill("Blue");
  await expect(page.getByText("Name is available!")).toBeVisible({
    timeout: 15_000,
  });
  await chooseCurrentMobileOption(page, "Choose Your Faction");
  await page
    .getByTestId("character-creation-navigation")
    .getByRole("button", { name: "Next", exact: true })
    .click();

  await expect(page.getByText("Step 2 of 4 · Class")).toBeVisible();
  const defaultClass = page
    .getByRole("button", { name: /Bug Catcher/i })
    .filter({ visible: true });
  await expect(defaultClass).toHaveAttribute("aria-pressed", "true");
  await expect(
    page.getByRole("button", { name: "Previous class" }),
  ).toHaveCount(0);
  await expectMobileViewportFits(page);
  await expectMobileNavigationFits(page);
  await expectChoiceListReachesNavigation(page, "Next");
  await page
    .getByTestId("character-creation-navigation")
    .getByRole("button", { name: "Next", exact: true })
    .click();

  await expect(page.getByText("Step 3 of 4 · Home")).toBeVisible();
  await expect
    .poll(() =>
      page
        .getByRole("region", { name: "Choose Your Home City" })
        .getByRole("button")
        .count(),
    )
    .toBeGreaterThan(1);
  await expectMobileViewportFits(page);
  await expectMobileNavigationFits(page);
  await expectChoiceListReachesNavigation(page, "Next");
  await page
    .getByTestId("character-creation-navigation")
    .getByRole("button", { name: "Next", exact: true })
    .click();

  await expect(page.getByText("Step 4 of 4 · Confirm")).toBeVisible();
  await expectMobileViewportFits(page);
  await expectMobileNavigationFits(page);
  await page.getByRole("button", { name: "Create", exact: true }).click();

  await expect(
    page.getByRole("heading", { name: "Select a Character" }),
  ).toBeVisible({ timeout: 30_000 });
  await expect(page.getByAltText(`${name} trainer preview`)).toBeVisible();
  await expectMobileViewportFits(page);
}

test("mobile character selection and creation stay compact", async ({
  page,
}) => {
  test.setTimeout(120_000);
  await page.setViewportSize({ width: 320, height: 568 });
  await page.goto("/");
  await expect(page.getByAltText("CaptureQuest")).toBeVisible();
  await expect
    .poll(() =>
      page
        .getByAltText("CaptureQuest")
        .locator("..")
        .evaluate((element) => getComputedStyle(element).marginTop),
    )
    .toBe("150px");
  await loginAsGuest(page, randomUUID());

  const firstCharacter = uniqueTrainerName();
  const secondCharacter = uniqueTrainerName();
  await createMobileCharacter(page, firstCharacter);
  await createMobileCharacter(page, secondCharacter);

  await expect(
    page.getByAltText(`${secondCharacter} trainer preview`),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: /Create New/i })).toHaveCount(
    1,
  );
  const createNewBounds = await page
    .getByRole("button", { name: "Create New", exact: true })
    .boundingBox();
  const enterWorldBounds = await page
    .getByRole("button", { name: "Enter World", exact: true })
    .boundingBox();
  expect(createNewBounds).not.toBeNull();
  expect(enterWorldBounds).not.toBeNull();
  expect(enterWorldBounds!.x).toBeGreaterThan(createNewBounds!.x);

  await page.getByRole("button", { name: "Previous character" }).click();
  await expect(
    page.getByAltText(`${firstCharacter} trainer preview`),
  ).toBeVisible();
  await expect(page.getByText(/Character [12] of 2/)).toBeVisible();
  await expectMobileViewportFits(page);

  await page.setViewportSize({ width: 844, height: 390 });
  await page.getByRole("button", { name: "Create New" }).click();
  await expect(page.getByText("Step 1 of 4 · Trainer")).toBeVisible();
  await expectMobileViewportFits(page);

  await page.getByPlaceholder("Enter character name").fill(uniqueTrainerName());
  await page.getByPlaceholder("Enter rival name").fill("Blue");
  await expect(page.getByText("Name is available!")).toBeVisible({
    timeout: 15_000,
  });
  await chooseCurrentMobileOption(page, "Choose Your Faction");
  await page
    .getByTestId("character-creation-navigation")
    .getByRole("button", { name: "Next", exact: true })
    .click();
  await expectMobileViewportFits(page);
  await expectMobileNavigationFits(page);
  await expectChoiceListReachesNavigation(page, "Next");
  await page
    .getByTestId("character-creation-navigation")
    .getByRole("button", { name: "Next", exact: true })
    .click();
  await expectMobileViewportFits(page);
  await expectMobileNavigationFits(page);
  await expectChoiceListReachesNavigation(page, "Next");
  await page
    .getByTestId("character-creation-navigation")
    .getByRole("button", { name: "Next", exact: true })
    .click();
  await expectMobileViewportFits(page);
  await expectMobileNavigationFits(page);
});

test("desktop character choices scroll inside their framed panels", async ({
  browser,
}) => {
  const context = await browser.newContext({
    baseURL:
      process.env.E2E_APP_URL ||
      process.env.INTEGRATION_APP_URL ||
      "http://localhost:5173",
    viewport: { width: 1440, height: 900 },
    hasTouch: false,
    isMobile: false,
  });
  const page = await context.newPage();

  try {
    await loginAsGuest(page, randomUUID());
    await page
      .getByRole("button", { name: "CREATE NEW CHARACTER" })
      .first()
      .click();

    const factionList = page.getByTestId("compact-choice-list");
    await expect(factionList).toBeVisible();
    const factionListBox = await factionList.boundingBox();
    const portraitBox = await page
      .locator("#CharacterCreator__ViewportContainer")
      .boundingBox();
    expect(factionListBox).not.toBeNull();
    expect(portraitBox).not.toBeNull();
    expect(Math.abs(factionListBox!.y - portraitBox!.y)).toBeLessThanOrEqual(2);
    expect(
      Math.abs(
        factionListBox!.y +
          factionListBox!.height -
          (portraitBox!.y + portraitBox!.height),
      ),
    ).toBeLessThanOrEqual(2);
    await expect
      .poll(() =>
        factionList.evaluate(
          (element) => getComputedStyle(element).borderImageSource,
        ),
      )
      .toContain("pokemon_frame/frame-hd.png");

    await page
      .getByPlaceholder("Enter character name")
      .fill(uniqueTrainerName());
    await page.getByPlaceholder("Enter rival name").fill("Blue");
    await expect(page.getByText("Name is available!")).toBeVisible({
      timeout: 15_000,
    });
    await factionList.getByRole("button").first().click();
    await page.getByRole("button", { name: "Next Step: Class" }).click();

    const classList = page.getByTestId("compact-choice-list");
    await expect(classList).toBeVisible();
    const classListBox = await classList.boundingBox();
    expect(classListBox).not.toBeNull();
    expect(
      Math.abs(classListBox!.height - factionListBox!.height),
    ).toBeLessThanOrEqual(2);
    await expect
      .poll(() =>
        classList.evaluate(
          (element) => element.scrollHeight > element.clientHeight,
        ),
      )
      .toBe(true);
    await expect
      .poll(() =>
        page
          .getByTestId("character-creator-step")
          .evaluate((element) => element.scrollHeight - element.clientHeight),
      )
      .toBe(0);

    const homeButton = page.getByRole("button", {
      name: "Next Step: Home City",
    });
    const homeButtonBox = await homeButton.boundingBox();
    expect(homeButtonBox).not.toBeNull();
    expect(homeButtonBox!.width).toBeGreaterThanOrEqual(330);
    expect(homeButtonBox!.x + homeButtonBox!.width).toBeLessThanOrEqual(1420);
    await homeButton.click();
    const confirmButton = page.getByRole("button", {
      name: "Next Step: Confirm",
    });
    const confirmButtonBox = await confirmButton.boundingBox();
    expect(confirmButtonBox).not.toBeNull();
    expect(confirmButtonBox!.x + confirmButtonBox!.width).toBeLessThanOrEqual(
      1420,
    );
    await expect
      .poll(() =>
        page
          .getByTestId("character-creator-step")
          .evaluate((element) => element.scrollHeight - element.clientHeight),
      )
      .toBe(0);
    await confirmButton.click();

    const confirmationOffset = await page
      .getByTestId("confirmation-content")
      .evaluate((element) => {
        const container = element.getBoundingClientRect();
        const story = element.lastElementChild?.getBoundingClientRect();
        if (!story) return null;
        return Math.abs(
          story.top + story.height / 2 - (container.top + container.height / 2),
        );
      });
    expect(confirmationOffset).not.toBeNull();
    expect(confirmationOffset!).toBeLessThanOrEqual(12);
  } finally {
    await context.close();
  }
});
