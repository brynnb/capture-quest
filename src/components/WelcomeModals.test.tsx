import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { WelcomeModals } from "./WelcomeModals";

const audioMocks = vi.hoisted(() => ({
  initialize: vi.fn(() => new Promise<void>(() => {})),
  playMusic: vi.fn(),
}));

vi.mock("@/services/audio/AudioManager", () => ({
  default: {
    isInitialized: () => false,
    initialize: audioMocks.initialize,
    playMusic: audioMocks.playMusic,
  },
}));

describe("WelcomeModals", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean })
      .IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
    vi.clearAllMocks();
    (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean })
      .IS_REACT_ACT_ENVIRONMENT = false;
  });

  it("dismisses on the first click without waiting for audio initialization", async () => {
    await act(async () => {
      root.render(<WelcomeModals />);
    });

    const button = Array.from(container.querySelectorAll("button")).find(
      (candidate) => candidate.textContent?.includes("Begin Your Journey"),
    );
    expect(button).toBeDefined();

    await act(async () => {
      button!.click();
      await Promise.resolve();
    });

    expect(audioMocks.initialize).toHaveBeenCalledTimes(1);
    expect(container.textContent).not.toContain("Begin Your Journey");
    expect(audioMocks.playMusic).not.toHaveBeenCalled();
  });
});
