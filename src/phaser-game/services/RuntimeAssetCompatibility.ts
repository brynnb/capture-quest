import { RUNTIME_TILE_ASSET_VERSION } from "@/constants/runtime_asset_version";

const RUNTIME_ASSET_CONTRACT_PATH = "/phaser/runtime_asset_contract.json";
const RUNTIME_ASSET_CONTRACT_TIMEOUT_MS = 10_000;
const VERIFICATION_TTL_MS = 15_000;
const RELOAD_GUARD_KEY = "capturequest:runtime-tile-catalog-reload";

interface RuntimeAssetContract {
  tileCatalogSha256: string;
}

export interface RuntimeAssetCompatibilityEnvironment {
  now: () => number;
  fetchCatalogHash: () => Promise<string>;
  currentUrl: () => string;
  readReloadGuard: () => string | null;
  writeReloadGuard: (catalogHash: string) => void;
  clearReloadGuard: () => void;
  replaceLocation: (url: string) => void;
}

export class RuntimeAssetReloadRequestedError extends Error {
  constructor() {
    super("Updating CaptureQuest to the current tile catalog");
    this.name = "RuntimeAssetReloadRequestedError";
  }
}

let lastVerifiedAt = 0;
let verificationInFlight: Promise<void> | null = null;

const browserEnvironment: RuntimeAssetCompatibilityEnvironment = {
  now: () => Date.now(),
  fetchCatalogHash: () => fetchRuntimeTileCatalogHash(),
  currentUrl: () => window.location.href,
  readReloadGuard: () => window.sessionStorage.getItem(RELOAD_GUARD_KEY),
  writeReloadGuard: (catalogHash) =>
    window.sessionStorage.setItem(RELOAD_GUARD_KEY, catalogHash),
  clearReloadGuard: () => window.sessionStorage.removeItem(RELOAD_GUARD_KEY),
  replaceLocation: (url) => window.location.replace(url),
};

function normalizedCatalogHash(value: unknown): string | null {
  if (typeof value !== "string" || !/^[0-9a-f]{64}$/i.test(value)) {
    return null;
  }
  return value.toLowerCase();
}

export async function fetchRuntimeTileCatalogHash(
  fetcher: typeof fetch = fetch,
): Promise<string> {
  const separator = RUNTIME_ASSET_CONTRACT_PATH.includes("?") ? "&" : "?";
  const abortController = new AbortController();
  let timeoutId: ReturnType<typeof setTimeout> | undefined;
  const timeout = new Promise<never>((_resolve, reject) => {
    timeoutId = setTimeout(() => {
      abortController.abort();
      reject(new Error("Runtime asset contract request timed out"));
    }, RUNTIME_ASSET_CONTRACT_TIMEOUT_MS);
  });
  let response: Response;
  try {
    response = await Promise.race([
      fetcher(
        `${RUNTIME_ASSET_CONTRACT_PATH}${separator}catalog-check=${Date.now()}`,
        {
          cache: "no-store",
          credentials: "same-origin",
          signal: abortController.signal,
        },
      ),
      timeout,
    ]);
  } finally {
    if (timeoutId !== undefined) clearTimeout(timeoutId);
  }
  if (!response.ok) {
    throw new Error(
      `Runtime asset contract request failed with HTTP ${response.status}`,
    );
  }
  const contract = (await response.json()) as Partial<RuntimeAssetContract>;
  const catalogHash = normalizedCatalogHash(contract.tileCatalogSha256);
  if (!catalogHash) {
    throw new Error("Runtime asset contract has an invalid tileCatalogSha256");
  }
  return catalogHash;
}

export function runtimeCatalogReloadUrl(
  currentUrl: string,
  catalogHash: string,
): string {
  const url = new URL(currentUrl);
  url.searchParams.set("asset-catalog", catalogHash.slice(0, 16));
  return url.toString();
}

export async function ensureRuntimeTileCatalogCurrent(
  force = false,
  environment: RuntimeAssetCompatibilityEnvironment = browserEnvironment,
): Promise<void> {
  if (!force && environment.now() - lastVerifiedAt < VERIFICATION_TTL_MS) {
    return;
  }
  if (verificationInFlight) return verificationInFlight;

  const verification = (async () => {
    const deployedHash = await environment.fetchCatalogHash();
    if (deployedHash === RUNTIME_TILE_ASSET_VERSION.toLowerCase()) {
      lastVerifiedAt = environment.now();
      try {
        environment.clearReloadGuard();
      } catch {
        // Storage can be unavailable in privacy modes; a matching bundle is
        // already safe, so there is nothing else to do.
      }
      return;
    }

    const reloadToken = deployedHash.slice(0, 16);
    let alreadyReloaded =
      new URL(environment.currentUrl()).searchParams.get("asset-catalog") ===
      reloadToken;
    try {
      alreadyReloaded =
        alreadyReloaded || environment.readReloadGuard() === deployedHash;
      if (!alreadyReloaded) {
        environment.writeReloadGuard(deployedHash);
      }
    } catch {
      // Continue with one cache-busted navigation. If storage is unavailable,
      // the changed URL still prevents a cached HTML response.
    }

    if (alreadyReloaded) {
      throw new Error(
        "The page bundle and deployed tile catalog still do not match after refresh. Please reload once deployment finishes.",
      );
    }

    environment.replaceLocation(
      runtimeCatalogReloadUrl(environment.currentUrl(), deployedHash),
    );
    throw new RuntimeAssetReloadRequestedError();
  })();
  verificationInFlight = verification;
  try {
    await verification;
  } finally {
    if (verificationInFlight === verification) verificationInFlight = null;
  }
}

/** Reset module-level TTL/coalescing state for hermetic unit tests only. */
export function resetRuntimeAssetCompatibilityForTests(): void {
  lastVerifiedAt = 0;
  verificationInFlight = null;
}
