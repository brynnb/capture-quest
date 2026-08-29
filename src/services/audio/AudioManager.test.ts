import { afterEach, describe, expect, it, vi } from "vitest";

interface FakeIOSAudioElement {
  muted: boolean;
  paused: boolean;
  removed: boolean;
  mutedWhenPlayStarted: boolean | null;
}

describe("AudioManager media muting", () => {
  afterEach(async () => {
    const { default: audioManager } = await import("./AudioManager");
    audioManager.stopMusic(false);
    audioManager.setMuted(false);
    vi.unstubAllGlobals();
  });

  it("uses native muted state before playback and updates fading tracks", async () => {
    const elements: FakeIOSAudioElement[] = [];

    class IOSAudioElement {
      muted = false;
      paused = false;
      removed = false;
      mutedWhenPlayStarted: boolean | null = null;
      crossOrigin = "";
      preload = "";
      src = "";
      loop = false;

      // Safari on iOS keeps media volume under physical user control; model
      // that behavior by ignoring JavaScript assignments and always reading 1.
      get volume() {
        return 1;
      }

      set volume(_value: number) {}

      constructor() {
        elements.push(this);
      }

      setAttribute() {}

      play() {
        this.mutedWhenPlayStarted = this.muted;
        return new Promise<void>(() => {});
      }

      pause() {
        this.paused = true;
      }

      remove() {
        this.removed = true;
      }
    }

    vi.stubGlobal("Audio", IOSAudioElement);
    vi.stubGlobal("requestAnimationFrame", vi.fn(() => 1));

    const { default: audioManager } = await import("./AudioManager");
    audioManager.setMuted(true);
    audioManager.playMusic("/assets/loading.mp3");

    expect(elements).toHaveLength(1);
    expect(elements[0].mutedWhenPlayStarted).toBe(true);
    expect(elements[0].muted).toBe(true);

    audioManager.setMuted(false);
    expect(elements[0].muted).toBe(false);

    audioManager.playMusic("/assets/characterselect.mp3");
    expect(elements).toHaveLength(2);
    expect(elements[1].mutedWhenPlayStarted).toBe(false);

    // The first track is still in its crossfade. Muting must cover both it and
    // the current track rather than only the latest HTMLAudioElement.
    audioManager.setMuted(true);
    expect(elements.map((element) => element.muted)).toEqual([true, true]);
  });
});
