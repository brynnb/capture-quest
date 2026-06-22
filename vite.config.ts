import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react-swc";
import path from "path";
import fs from "fs";
import tsconfigPaths from "vite-tsconfig-paths";
import type { Plugin } from "vite";

const verboseDebugLog = process.env.VERBOSE_DEBUG_LOG === "true";

function devServerPort(): number {
  const rawPort = process.env.VITE_DEV_PORT;
  if (!rawPort) return 5173;
  const parsed = Number.parseInt(rawPort, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 5173;
}

function hashServerPort(): number {
  const rawPort = process.env.VITE_HASH_PORT || process.env.HASH_PORT;
  if (!rawPort) return 7100;
  const parsed = Number.parseInt(rawPort, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 7100;
}

function apiServerPort(): number {
  const rawPort = process.env.VITE_API_PORT || process.env.HTTP_PORT;
  if (!rawPort) return 8080;
  const parsed = Number.parseInt(rawPort, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 8080;
}

// Middleware to proxy /api/hash to the Go server
// This avoids CORS/TLS issues by fetching from same origin
function hashProxyPlugin(): Plugin {
  return {
    name: "hash-proxy",
    configureServer({ middlewares }) {
      middlewares.use(async (req, res, next) => {
        if (req.url?.startsWith("/api/hash")) {
          try {
            const response = await fetch(`http://127.0.0.1:${hashServerPort()}/hash`);
            if (response.ok) {
              const hash = await response.text();
              res.setHeader("Content-Type", "text/plain");
              res.end(hash);
            } else {
              res.statusCode = 502;
              res.end("Failed to fetch hash from Go server");
            }
          } catch (err) {
            if (verboseDebugLog) {
              console.warn("Hash proxy error:", err);
            }
            res.statusCode = 502;
            res.end("Failed to connect to Go server hash endpoint");
          }
          return;
        }
        next();
      });
    },
  };
}

function localAssetsPlugin(): Plugin {
  return {
    name: "local-assets-proxy",
    configureServer({ middlewares }) {
      const isOffline = process.env.VITE_OFFLINE_ASSETS === 'true';
      const assetsPath = path.resolve(process.cwd(), 'capturequest_assets/capturequest');

      if (isOffline && fs.existsSync(assetsPath)) {
        console.log(`[Vite] Serving local assets from: ${assetsPath}`);

        middlewares.use('/capturequest', (req, res, next) => {
          // Strip /capturequest from URL
          const urlStr = req.url?.split('?')[0] || '';
          const filePath = path.join(assetsPath, urlStr);

          if (fs.existsSync(filePath) && fs.statSync(filePath).isFile()) {
            const content = fs.readFileSync(filePath);

            // Set basic mime types
            if (filePath.endsWith('.gz')) res.setHeader('Content-Encoding', 'gzip');
            if (filePath.endsWith('.glb')) res.setHeader('Content-Type', 'model/gltf-binary');

            res.end(content);
            return;
          }

          // If it's an /offline_assets request but file is missing, return 404
          // to prevent Vite from serving index.html as a fallback.
          res.statusCode = 404;
          res.end('Not Found');
        });
      }
    },
  };
}

// Middleware to proxy /api/tiles/* to the Go server
function tileArtProxyPlugin(): Plugin {
  return {
    name: "tile-art-proxy",
    configureServer({ middlewares }) {
      middlewares.use(async (req, res, next) => {
        if (!req.url?.startsWith("/api/tiles/")) return next();

        try {
          const goUrl = `http://127.0.0.1:${apiServerPort()}${req.url}`;
          const method = req.method || "GET";

          // For POST requests, pipe the body through
          if (method === "POST") {
            const chunks: Buffer[] = [];
            req.on("data", (chunk: Buffer) => chunks.push(chunk));
            req.on("end", async () => {
              try {
                const body = Buffer.concat(chunks);
                const response = await fetch(goUrl, {
                  method: "POST",
                  headers: {
                    "content-type": req.headers["content-type"] || "",
                  },
                  body,
                });
                const text = await response.text();
                res.setHeader("Content-Type", "application/json");
                res.statusCode = response.status;
                res.end(text);
              } catch (err) {
                if (verboseDebugLog) console.warn("Tile art proxy POST error:", err);
                res.statusCode = 502;
                res.end(JSON.stringify({ success: false, error: "Failed to connect to Go server" }));
              }
            });
          } else {
            const response = await fetch(goUrl);
            const text = await response.text();
            res.setHeader("Content-Type", "application/json");
            res.statusCode = response.status;
            res.end(text);
          }
        } catch (err) {
          if (verboseDebugLog) console.warn("Tile art proxy error:", err);
          res.statusCode = 502;
          res.end(JSON.stringify({ success: false, error: "Failed to connect to Go server" }));
        }
      });
    },
  };
}

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    react({ plugins: [["@swc/plugin-styled-components", {}]] }),
    tsconfigPaths(),
    hashProxyPlugin(),
    tileArtProxyPlugin(),
    localAssetsPlugin(),
    // Required for SharedArrayBuffer and OPFS access
    {
      name: 'coop-coep',
      configureServer: ({ middlewares }) => {
        middlewares.use((_req, res, next) => {
          res.setHeader("Cross-Origin-Embedder-Policy", "require-corp");
          res.setHeader("Cross-Origin-Opener-Policy", "same-origin");
          next();
        });
      },
    },
  ],

  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
      "@utils": path.resolve(__dirname, "./src/utils"),
      "@data": path.resolve(__dirname, "./data"),
      "@stores": path.resolve(__dirname, "./src/stores"),
      "@hooks": path.resolve(__dirname, "./src/hooks"),
      "@components": path.resolve(__dirname, "./src/components"),
      "@entities": path.resolve(__dirname, "./src/entities"),
      "@scripts": path.resolve(__dirname, "./src/scripts"),
      "@pages": path.resolve(__dirname, "./src/pages"),
      "@interfaces": path.resolve(__dirname, "./src/interfaces"),
      "@net": path.resolve(__dirname, "./src/net"),
    },
  },
  test: {
    globals: true,
    setupFiles: ["./tests/testSetup.ts"],
    environment: "jsdom",
    include: ["./tests/**/*.{test,spec}.{js,mjs,cjs,ts,mts,cts,jsx,tsx}"],
  },
  server: {
    port: devServerPort(),
    strictPort: false,
    watch: {
      ignored: [
        "**/public/assets/pokemon/**",
        "**/public/assets/pokemon_frame/**",
        "**/public/assets/trainers/**",
        "**/public/phaser/assets/**",
        "**/public/phaser/sprites/**",
        "**/public/phaser/tile_images/**",
        "**/public/sound/**",
      ],
    },
    proxy: {
      "/ws": {
        target: "ws://127.0.0.1:8080",
        ws: true,
      },
    },
  },
});
