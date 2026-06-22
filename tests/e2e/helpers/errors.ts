import { expect, type Page } from "@playwright/test";

const ignoredConsoleFragments = [
  "Failed to load resource: the server responded with a status of 404",
  "Failed to load resource: the server responded with a status of 500",
  "Failed to load resource: the server responded with a status of 502",
  "Failed to load resource: the server responded with a status of 503",
  "Failed to load resource: the server responded with a status of 504",
  "Failed to load resource: net::ERR_NETWORK_CHANGED",
  "Pokemon font failed to load",
  "WebGL warning:",
  "Alpha-premult and y-flip are deprecated",
  "generateMipmap: Tex image",
  "favicon",
];

export interface PageErrorCollector {
  consoleErrors: string[];
  pageErrors: string[];
  networkErrors: string[];
  assertNoSevereErrors: () => void;
}

function isIgnored(message: string): boolean {
  return ignoredConsoleFragments.some((fragment) => message.includes(fragment));
}

export function collectPageErrors(page: Page): PageErrorCollector {
  const consoleErrors: string[] = [];
  const pageErrors: string[] = [];
  const networkErrors: string[] = [];

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

  page.on("response", (response) => {
    const status = response.status();
    if (status < 500) return;

    networkErrors.push(`${status} ${response.url()}`);
  });

  return {
    consoleErrors,
    pageErrors,
    networkErrors,
    assertNoSevereErrors: () => {
      expect([...consoleErrors, ...pageErrors, ...networkErrors]).toEqual([]);
    },
  };
}
