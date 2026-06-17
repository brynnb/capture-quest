import { expect, type Page } from "@playwright/test";

const ignoredConsoleFragments = [
  "Failed to load resource: the server responded with a status of 404",
  "Pokemon font failed to load",
  "WebGL warning:",
  "Alpha-premult and y-flip are deprecated",
  "generateMipmap: Tex image",
  "favicon",
];

export interface PageErrorCollector {
  consoleErrors: string[];
  pageErrors: string[];
  assertNoSevereErrors: () => void;
}

function isIgnored(message: string): boolean {
  return ignoredConsoleFragments.some((fragment) => message.includes(fragment));
}

export function collectPageErrors(page: Page): PageErrorCollector {
  const consoleErrors: string[] = [];
  const pageErrors: string[] = [];

  page.on("console", (message) => {
    if (message.type() !== "error") return;
    const text = message.text();
    if (!isIgnored(text)) {
      consoleErrors.push(text);
    }
  });

  page.on("pageerror", (error) => {
    const message = error.message;
    if (!isIgnored(message)) {
      pageErrors.push(message);
    }
  });

  return {
    consoleErrors,
    pageErrors,
    assertNoSevereErrors: () => {
      expect([...consoleErrors, ...pageErrors]).toEqual([]);
    },
  };
}
