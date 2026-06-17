#!/usr/bin/env node

import { spawnSync } from "node:child_process";
import { createRequire } from "node:module";
import { resolve } from "node:path";

const require = createRequire(import.meta.url);

const cwd = resolve(import.meta.dirname, "../..");
const playwrightVersion = require("@playwright/test/package.json").version;
const image =
  process.env.PLAYWRIGHT_DOCKER_IMAGE ||
  `mcr.microsoft.com/playwright:v${playwrightVersion}-noble`;
const dockerBinary = process.env.DOCKER || "docker";
const dockerNetwork = process.env.PLAYWRIGHT_DOCKER_NETWORK || "host";
const useHostNetwork = dockerNetwork === "host";
const appHostName = useHostNetwork ? "localhost" : "host.docker.internal";
const appUrl = process.env.E2E_APP_URL || `http://${appHostName}:5173`;
const extraArgs = process.argv.slice(2);

const readNumericEnv = (key) => {
  const value = Number.parseInt(process.env[key] ?? "", 10);
  return Number.isInteger(value) ? value : null;
};

const uid =
  readNumericEnv("SUDO_UID") ??
  (typeof process.getuid === "function" ? process.getuid() : null);
const gid =
  readNumericEnv("SUDO_GID") ??
  (typeof process.getgid === "function" ? process.getgid() : null);

const dockerArgs = [
  "run",
  "--rm",
  "--ipc",
  "host",
  "-v",
  `${cwd}:/work`,
  "-w",
  "/work",
  "-e",
  "HOME=/tmp",
  "-e",
  `E2E_APP_URL=${appUrl}`,
  "-e",
  `INTEGRATION_APP_URL=${appUrl}`,
];

if (useHostNetwork) {
  dockerArgs.splice(2, 0, "--network", "host");
} else {
  dockerArgs.splice(2, 0, "--add-host", "host.docker.internal:host-gateway");
}

if (uid != null && gid != null) {
  dockerArgs.push("--user", `${uid}:${gid}`);
}

dockerArgs.push(
  image,
  "npx",
  "playwright",
  "test",
  "-c",
  "playwright.config.ts",
  ...extraArgs,
);

const result = spawnSync(dockerBinary, dockerArgs, {
  cwd,
  stdio: "inherit",
});

if (result.error) {
  if (result.error.code === "ENOENT") {
    console.error("Docker is not installed or is not on PATH.");
  } else {
    console.error(result.error.message);
  }
  process.exit(1);
}

process.exit(result.status ?? 1);
