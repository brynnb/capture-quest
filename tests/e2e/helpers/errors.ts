import { expect, type Page } from "@playwright/test";
import { PhaserPlayerPositionUpdate } from "../../../src/net/generated/opcodes";

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
  retiredPositionPackets: string[];
  sentOpcodes: number[];
  assertNoSevereErrors: () => void;
}

function isIgnored(message: string): boolean {
  return ignoredConsoleFragments.some((fragment) => message.includes(fragment));
}

export function collectPageErrors(page: Page): PageErrorCollector {
  const consoleErrors: string[] = [];
  const pageErrors: string[] = [];
  const networkErrors: string[] = [];
  const retiredPositionPackets: string[] = [];
  const sentOpcodes: number[] = [];

  page.on("websocket", (socket) => {
    if (new URL(socket.url()).pathname !== "/ws") return;
    socket.on("framesent", ({ payload }) => {
      // capturequest-socket wraps both datagrams and streams as
      // [length:uint32_LE][opcode:uint16_LE][JSON] on the WebSocket transport.
      if (typeof payload === "string" || payload.length < 6) return;
      if (payload.readUInt32LE(0) !== payload.length - 4) return;
      const opcode = payload.readUInt16LE(4);
      sentOpcodes.push(opcode);
      if (opcode === PhaserPlayerPositionUpdate) {
        retiredPositionPackets.push(payload.subarray(6).toString("utf8"));
      }
    });
  });

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
    retiredPositionPackets,
    sentOpcodes,
    assertNoSevereErrors: () => {
      expect([...consoleErrors, ...pageErrors, ...networkErrors]).toEqual([]);
      expect(retiredPositionPackets, "client emitted a retired coordinate setter").toEqual([]);
    },
  };
}
