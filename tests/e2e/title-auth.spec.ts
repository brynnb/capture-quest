import { expect, test } from "@playwright/test";

test("title screen exposes login, register, and guest entry points", async ({
  page,
}) => {
  await page.goto("/");

  await expect(page.getByAltText("CaptureQuest")).toBeVisible();
  await expect(page.getByRole("button", { name: "LOGIN" })).toBeVisible();
  await expect(page.getByRole("button", { name: "REGISTER" })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "PLAY AS GUEST" }),
  ).toBeVisible();
});

test("login and register flows can be opened and returned from", async ({
  page,
}) => {
  await page.goto("/");

  await page.getByRole("button", { name: "LOGIN" }).click();
  await expect(page.getByRole("heading", { name: "Login" })).toBeVisible();
  await expect(page.getByPlaceholder("Enter email address")).toBeVisible();
  await expect(page.getByPlaceholder("Enter password")).toBeVisible();
  await page.getByRole("button", { name: "BACK" }).click();

  await page.getByRole("button", { name: "REGISTER" }).click();
  await expect(
    page.getByRole("heading", { name: "Create Account" }),
  ).toBeVisible();
  await expect(page.getByPlaceholder("Enter email address")).toBeVisible();
  await expect(page.getByPlaceholder("Enter password")).toBeVisible();
  await expect(page.getByPlaceholder("Confirm password")).toBeVisible();
});
