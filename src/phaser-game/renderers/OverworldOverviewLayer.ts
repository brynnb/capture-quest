import type { Scene } from "phaser";

import { RUNTIME_TILE_ASSET_VERSION } from "@/constants/runtime_asset_version";
import { getApiUrl } from "@/config";
import { TILE_SIZE } from "../constants";
import { ensureRuntimeTileCatalogCurrent } from "../services/RuntimeAssetCompatibility";
import { refreshWorldTextureSampling } from "./worldTextureSampling";

export interface OverviewChunkRef {
  key: string;
  column: number;
  row: number;
}

interface OverviewChunkEntry {
  image: Phaser.GameObjects.Image;
  textureKey: string;
  revision: number;
}

interface OverviewSyncJob {
  generation: number;
  remaining: number;
  ready: boolean;
  firstError: unknown;
  resolve: () => void;
  reject: (error: unknown) => void;
  onBackgroundFailure?: (error: unknown) => void;
  backgroundFailureReported: boolean;
  readinessTimeoutId?: ReturnType<typeof setTimeout>;
}

interface QueuedOverviewLoad {
  chunk: OverviewChunkRef;
  revision: number;
  job: OverviewSyncJob;
}

const OVERVIEW_LOAD_CONCURRENCY = 4;
const OVERVIEW_FILE_XHR_TIMEOUT_MS = 12_000;
const OVERVIEW_LOAD_TIMEOUT_MS = 15_000;
const OVERVIEW_INITIAL_READINESS_TIMEOUT_MS = 16_000;
export const OVERWORLD_OVERVIEW_IMAGE_NAME_PREFIX = "overworld-overview:";

export class OverworldOverviewLayer {
  private readonly entries = new Map<string, OverviewChunkEntry>();
  private readonly pendingLoads = new Map<string, Promise<string>>();
  private readonly desiredTextureKeyByChunk = new Map<string, string>();
  private readonly loadQueue: QueuedOverviewLoad[] = [];
  private activeLoadCount = 0;
  private visible = false;
  private syncGeneration = 0;
  private cancelActiveReadiness: (() => void) | null = null;

  constructor(
    private readonly scene: Scene,
    private readonly mapContainer: Phaser.GameObjects.Container,
    private readonly chunkSize: number,
  ) {}

  async sync(
    chunks: readonly OverviewChunkRef[],
    revisionForKey: (key: string) => number,
    onBackgroundFailure?: (error: unknown) => void,
  ): Promise<void> {
    this.cancelActiveReadiness?.();
    this.cancelActiveReadiness = null;
    const generation = ++this.syncGeneration;
    this.loadQueue.splice(0);
    const desiredKeys = new Set(chunks.map((chunk) => chunk.key));
    this.desiredTextureKeyByChunk.clear();
    for (const chunk of chunks) {
      this.desiredTextureKeyByChunk.set(
        chunk.key,
        this.textureKey(chunk, revisionForKey(chunk.key)),
      );
    }
    for (const key of this.entries.keys()) {
      if (!desiredKeys.has(key)) this.remove(key);
    }

    const pendingChunks = chunks.filter((chunk) => {
      const entry = this.entries.get(chunk.key);
      return entry?.revision !== revisionForKey(chunk.key);
    });
    if (pendingChunks.length === 0) return;

    const alreadyReady = chunks.some((chunk) => {
      const entry = this.entries.get(chunk.key);
      return entry?.revision === revisionForKey(chunk.key);
    });
    let resolveReadiness: () => void = () => undefined;
    let rejectReadiness: (error: unknown) => void = () => undefined;
    const readiness = alreadyReady
      ? Promise.resolve()
      : new Promise<void>((resolve, reject) => {
          resolveReadiness = resolve;
          rejectReadiness = reject;
        });
    const job: OverviewSyncJob = {
      generation,
      remaining: pendingChunks.length,
      ready: alreadyReady,
      firstError: undefined,
      resolve: resolveReadiness,
      reject: rejectReadiness,
      onBackgroundFailure,
      backgroundFailureReported: false,
    };
    if (!alreadyReady) {
      this.cancelActiveReadiness = () => {
        if (job.ready) return;
        job.ready = true;
        if (job.readinessTimeoutId !== undefined) {
          clearTimeout(job.readinessTimeoutId);
        }
        job.resolve();
      };
      job.readinessTimeoutId = setTimeout(() => {
        if (job.ready || job.generation !== this.syncGeneration) return;
        job.ready = true;
        this.cancelActiveReadiness = null;
        this.removeQueuedJob(job);
        // Active files are independently bounded by their XHR and watchdog
        // timeouts. Superseding the generation prevents their late pixels from
        // becoming owned layer entries after initial readiness has failed.
        this.syncGeneration += 1;
        job.reject(
          new Error("Timed out waiting for an overworld overview chunk"),
        );
      }, OVERVIEW_INITIAL_READINESS_TIMEOUT_MS);
    }

    this.loadQueue.push(
      ...pendingChunks.map((chunk) => ({
        chunk,
        revision: revisionForKey(chunk.key),
        job,
      })),
    );
    this.pumpLoadQueue();

    await readiness;
  }

  setVisible(visible: boolean): void {
    this.visible = visible;
    for (const entry of this.entries.values()) {
      entry.image.setVisible(visible);
    }
  }

  invalidate(key: string): void {
    this.remove(key);
  }

  clear(): void {
    this.cancelActiveReadiness?.();
    this.cancelActiveReadiness = null;
    this.syncGeneration += 1;
    this.loadQueue.splice(0);
    this.desiredTextureKeyByChunk.clear();
    for (const key of [...this.entries.keys()]) this.remove(key);
  }

  private async ensure(
    chunk: OverviewChunkRef,
    revision: number,
    generation: number,
  ): Promise<void> {
    const existing = this.entries.get(chunk.key);
    if (existing?.revision === revision) return;
    if (existing) this.remove(chunk.key);

    const textureKey = await this.loadTexture(chunk, revision);
    if (generation !== this.syncGeneration) {
      if (
        this.desiredTextureKeyByChunk.get(chunk.key) !== textureKey &&
        this.scene.textures.exists(textureKey)
      ) {
        this.scene.textures.remove(textureKey);
      }
      return;
    }
    if (this.entries.has(chunk.key)) return;

    refreshWorldTextureSampling(
      this.scene,
      this.scene.textures.get(textureKey),
    );

    const originX = chunk.column * this.chunkSize;
    const originY = chunk.row * this.chunkSize;
    const image = this.scene.add.image(
      originX * TILE_SIZE,
      originY * TILE_SIZE,
      textureKey,
    );
    image.setOrigin(0, 0);
    image.setDisplaySize(
      this.chunkSize * TILE_SIZE,
      this.chunkSize * TILE_SIZE,
    );
    image.setName(`${OVERWORLD_OVERVIEW_IMAGE_NAME_PREFIX}${chunk.key}`);
    image.setDepth(-10);
    image.setVisible(this.visible);
    this.mapContainer.add(image);
    this.mapContainer.sendToBack(image);
    this.entries.set(chunk.key, { image, textureKey, revision });
  }

  private pumpLoadQueue(): void {
    while (
      this.activeLoadCount < OVERVIEW_LOAD_CONCURRENCY &&
      this.loadQueue.length > 0
    ) {
      const queued = this.loadQueue.shift();
      if (!queued) return;
      if (queued.job.generation !== this.syncGeneration) continue;

      this.activeLoadCount += 1;
      void this.ensure(queued.chunk, queued.revision, queued.job.generation)
        .then(() => {
          if (
            queued.job.generation === this.syncGeneration &&
            this.entries.get(queued.chunk.key)?.revision === queued.revision &&
            !queued.job.ready
          ) {
            queued.job.ready = true;
            if (queued.job.readinessTimeoutId !== undefined) {
              clearTimeout(queued.job.readinessTimeoutId);
            }
            this.cancelActiveReadiness = null;
            queued.job.resolve();
            this.reportBackgroundFailure(queued.job);
          }
        })
        .catch((error: unknown) => {
          queued.job.firstError ??= error;
          if (queued.job.generation === this.syncGeneration) {
            console.warn(
              `[OverworldOverview] Failed to load chunk ${queued.chunk.key}:`,
              error,
            );
          }
          if (
            queued.job.ready &&
            queued.job.generation === this.syncGeneration
          ) {
            this.reportBackgroundFailure(queued.job);
          }
        })
        .finally(() => {
          queued.job.remaining -= 1;
          if (
            queued.job.generation === this.syncGeneration &&
            queued.job.remaining === 0 &&
            !queued.job.ready
          ) {
            queued.job.ready = true;
            if (queued.job.readinessTimeoutId !== undefined) {
              clearTimeout(queued.job.readinessTimeoutId);
            }
            this.cancelActiveReadiness = null;
            queued.job.reject(
              queued.job.firstError ??
                new Error("No overworld overview chunk became ready"),
            );
          }
          this.activeLoadCount -= 1;
          this.pumpLoadQueue();
        });
    }
  }

  private removeQueuedJob(job: OverviewSyncJob): void {
    for (let index = this.loadQueue.length - 1; index >= 0; index -= 1) {
      if (this.loadQueue[index].job === job) this.loadQueue.splice(index, 1);
    }
  }

  private reportBackgroundFailure(job: OverviewSyncJob): void {
    if (
      job.backgroundFailureReported ||
      job.firstError === undefined ||
      !job.onBackgroundFailure
    ) {
      return;
    }
    job.backgroundFailureReported = true;
    job.onBackgroundFailure(job.firstError);
  }

  private loadTexture(
    chunk: OverviewChunkRef,
    revision: number,
  ): Promise<string> {
    const textureKey = this.textureKey(chunk, revision);
    if (this.scene.textures.exists(textureKey)) {
      return Promise.resolve(textureKey);
    }

    const pending = this.pendingLoads.get(textureKey);
    if (pending) return pending;

    const query = new URLSearchParams({
      chunkX: chunk.column.toString(),
      chunkY: chunk.row.toString(),
      catalog: RUNTIME_TILE_ASSET_VERSION,
      revision: revision.toString(),
    });
    const url = `${getApiUrl("/overworld/overview")}?${query.toString()}`;

    const load = new Promise<string>((resolve, reject) => {
      const completeEvent = `filecomplete-image-${textureKey}`;
      const timeoutId = setTimeout(() => {
        cleanup();
        reject(
          new Error(`Timed out loading overworld overview chunk ${chunk.key}`),
        );
      }, OVERVIEW_LOAD_TIMEOUT_MS);
      const cleanup = () => {
        clearTimeout(timeoutId);
        this.scene.load.off(completeEvent, onComplete);
        this.scene.load.off("loaderror", onError);
        this.pendingLoads.delete(textureKey);
      };
      const onComplete = () => {
        cleanup();
        resolve(textureKey);
      };
      const onError = (file: Phaser.Loader.File) => {
        if (file.key !== textureKey) return;
        cleanup();
        void ensureRuntimeTileCatalogCurrent(true).then(
          () =>
            reject(
              new Error(`Failed to load overworld overview chunk ${chunk.key}`),
            ),
          (error: unknown) => reject(error),
        );
      };

      this.scene.load.once(completeEvent, onComplete);
      this.scene.load.on("loaderror", onError);
      this.scene.load.image(textureKey, url, {
        // Abort the underlying request before the application-level watchdog.
        // The watchdog remains necessary for loader implementations that emit
        // neither completion nor loaderror after an abort.
        responseType: "blob",
        timeout: OVERVIEW_FILE_XHR_TIMEOUT_MS,
      });
      if (!this.scene.load.isLoading()) this.scene.load.start();
    });
    this.pendingLoads.set(textureKey, load);
    return load;
  }

  private textureKey(chunk: OverviewChunkRef, revision: number): string {
    return `overworld-overview-${chunk.column}-${chunk.row}-${revision}`;
  }

  private remove(key: string): void {
    const entry = this.entries.get(key);
    if (!entry) return;
    entry.image.destroy();
    this.entries.delete(key);
    if (this.scene.textures.exists(entry.textureKey)) {
      this.scene.textures.remove(entry.textureKey);
    }
  }
}
