import { afterEach, describe, expect, it, vi } from "vitest";

import { RUNTIME_TILE_ASSET_VERSION } from "@/constants/runtime_asset_version";

import {
  RuntimeAssetReloadRequestedError,
  ensureRuntimeTileCatalogCurrent,
  fetchRuntimeTileCatalogHash,
  resetRuntimeAssetCompatibilityForTests,
  runtimeCatalogReloadUrl,
  type RuntimeAssetCompatibilityEnvironment,
} from "./RuntimeAssetCompatibility";

afterEach(() => {
  vi.useRealTimers();
  resetRuntimeAssetCompatibilityForTests();
});

function compatibilityEnvironment(fetchCatalogHash: () => Promise<string>) {
  let currentUrl = "https://capturequest.net/play?release=red#map";
  let reloadGuard: string | null = null;
  const replaceLocation = vi.fn((url: string) => {
    currentUrl = url;
  });
  const environment: RuntimeAssetCompatibilityEnvironment = {
    now: () => 20_000,
    fetchCatalogHash,
    currentUrl: () => currentUrl,
    readReloadGuard: () => reloadGuard,
    writeReloadGuard: (catalogHash) => {
      reloadGuard = catalogHash;
    },
    clearReloadGuard: () => {
      reloadGuard = null;
    },
    replaceLocation,
  };
  return { environment, replaceLocation };
}

describe("runtime tile catalog compatibility", () => {
  it("fetches the published contract without accepting browser cache", async () => {
    const fetcher = vi.fn<typeof fetch>(async () =>
      Promise.resolve(
        new Response(JSON.stringify({ tileCatalogSha256: "A".repeat(64) }), {
          status: 200,
        }),
      ),
    );

    await expect(fetchRuntimeTileCatalogHash(fetcher)).resolves.toBe(
      "a".repeat(64),
    );
    expect(fetcher).toHaveBeenCalledOnce();
    const [url, options] = fetcher.mock.calls[0];
    expect(url).toMatch(
      /^\/phaser\/runtime_asset_contract\.json\?catalog-check=\d+$/,
    );
    expect(options).toEqual({
      cache: "no-store",
      credentials: "same-origin",
      signal: expect.any(AbortSignal),
    });
  });

  it("times out a runtime contract request that never settles", async () => {
    vi.useFakeTimers();
    const fetcher = vi.fn<typeof fetch>(() => new Promise(() => undefined));

    const loading = fetchRuntimeTileCatalogHash(fetcher);
    const rejection = expect(loading).rejects.toThrow(
      "Runtime asset contract request timed out",
    );
    await vi.runOnlyPendingTimersAsync();

    await rejection;
    const options = fetcher.mock.calls[0][1];
    expect(options?.signal?.aborted).toBe(true);
  });

  it("rejects missing and malformed deployed catalog hashes", async () => {
    const malformed = vi.fn<typeof fetch>(async () =>
      Promise.resolve(
        new Response(JSON.stringify({ tileCatalogSha256: "wrong" }), {
          status: 200,
        }),
      ),
    );
    await expect(fetchRuntimeTileCatalogHash(malformed)).rejects.toThrow(
      "invalid tileCatalogSha256",
    );

    const missing = vi.fn<typeof fetch>(async () =>
      Promise.resolve(new Response("missing", { status: 404 })),
    );
    await expect(fetchRuntimeTileCatalogHash(missing)).rejects.toThrow(
      "HTTP 404",
    );
  });

  it("builds a cache-busted reload URL without losing route state", () => {
    expect(
      runtimeCatalogReloadUrl(
        "https://capturequest.net/play?release=red#map",
        "1234567890abcdef" + "0".repeat(48),
      ),
    ).toBe(
      "https://capturequest.net/play?release=red&asset-catalog=1234567890abcdef#map",
    );
  });

  it("requests one cache-busted reload and then refuses a reload loop", async () => {
    const deployedHash = "1".repeat(64);
    const { environment, replaceLocation } = compatibilityEnvironment(
      async () => deployedHash,
    );

    await expect(
      ensureRuntimeTileCatalogCurrent(true, environment),
    ).rejects.toBeInstanceOf(RuntimeAssetReloadRequestedError);
    expect(replaceLocation).toHaveBeenCalledOnce();
    expect(replaceLocation).toHaveBeenCalledWith(
      expect.stringContaining("asset-catalog=1111111111111111"),
    );

    await expect(
      ensureRuntimeTileCatalogCurrent(true, environment),
    ).rejects.toThrow("still do not match after refresh");
    expect(replaceLocation).toHaveBeenCalledOnce();
  });

  it("coalesces concurrent checks and reuses a matching verification within the TTL", async () => {
    let resolveHash!: (hash: string) => void;
    const fetchCatalogHash = vi.fn(
      () =>
        new Promise<string>((resolve) => {
          resolveHash = resolve;
        }),
    );
    const { environment, replaceLocation } =
      compatibilityEnvironment(fetchCatalogHash);

    const first = ensureRuntimeTileCatalogCurrent(false, environment);
    const second = ensureRuntimeTileCatalogCurrent(false, environment);
    expect(fetchCatalogHash).toHaveBeenCalledOnce();
    resolveHash(RUNTIME_TILE_ASSET_VERSION.toLowerCase());
    await Promise.all([first, second]);

    await ensureRuntimeTileCatalogCurrent(false, environment);
    expect(fetchCatalogHash).toHaveBeenCalledOnce();
    expect(replaceLocation).not.toHaveBeenCalled();
  });
});
