#!/usr/bin/env node
import dgram from "node:dgram";
import net from "node:net";

const DEFAULT_HTTP_PORT = 8080;
const DEFAULT_HASH_PORT = 7100;
const DEFAULT_WT_PORT = 4433;
const DEFAULT_VITE_PORT = 5173;
const MAX_SCAN_STEPS = 100;

function requestedPort(name, fallback) {
  const value = Number.parseInt(process.env[name] || "", 10);
  return Number.isFinite(value) && value > 0 ? value : fallback;
}

function shellQuote(value) {
  return `'${String(value).replaceAll("'", "'\\''")}'`;
}

function tcpPortAvailable(port) {
  return new Promise((resolve) => {
    const server = net.createServer();
    server.once("error", () => resolve(false));
    server.once("listening", () => {
      server.close(() => resolve(true));
    });
    server.listen(port, "0.0.0.0");
  });
}

function udpPortAvailable(port) {
  return new Promise((resolve) => {
    const socket = dgram.createSocket("udp4");
    socket.once("error", () => {
      socket.close();
      resolve(false);
    });
    socket.once("listening", () => {
      socket.close(() => resolve(true));
    });
    socket.bind(port, "0.0.0.0");
  });
}

async function nextAvailablePort(startPort, isAvailable) {
  for (let offset = 0; offset <= MAX_SCAN_STEPS; offset++) {
    const candidate = startPort + offset;
    if (await isAvailable(candidate)) {
      return candidate;
    }
  }
  throw new Error(`No available port found from ${startPort} to ${startPort + MAX_SCAN_STEPS}`);
}

const httpPort = await nextAvailablePort(
  requestedPort("HTTP_PORT", DEFAULT_HTTP_PORT),
  tcpPortAvailable,
);
const hashPort = await nextAvailablePort(
  requestedPort("HASH_PORT", DEFAULT_HASH_PORT),
  tcpPortAvailable,
);
const wtPort = await nextAvailablePort(
  requestedPort("WT_PORT", DEFAULT_WT_PORT),
  udpPortAvailable,
);
const vitePort = await nextAvailablePort(
  requestedPort("VITE_DEV_PORT", DEFAULT_VITE_PORT),
  tcpPortAvailable,
);

const exports = {
  HTTP_PORT: httpPort,
  VITE_API_PORT: httpPort,
  HASH_PORT: hashPort,
  VITE_HASH_PORT: hashPort,
  WT_PORT: wtPort,
  VITE_WT_PORT: wtPort,
  VITE_DEV_PORT: vitePort,
};

for (const [key, value] of Object.entries(exports)) {
  console.log(`export ${key}=${shellQuote(value)}`);
}
