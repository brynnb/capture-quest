import { describe, expect, it, vi } from "vitest";
import { normalizePhaserArrayPayload } from "./PhaserNetworkService";

describe("normalizePhaserArrayPayload", () => {
  it("preserves valid list responses", () => {
    const payload = [{ id: 1 }, { id: 2 }];

    expect(normalizePhaserArrayPayload(payload, "warps response")).toBe(
      payload,
    );
  });

  it("turns a server error object into a safe empty list", () => {
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {});

    expect(
      normalizePhaserArrayPayload(
        { success: false, error: "database query failed" },
        "warps response",
      ),
    ).toEqual([]);
    expect(consoleError).toHaveBeenCalledWith(
      expect.stringContaining("database query failed"),
      expect.any(Object),
    );

    consoleError.mockRestore();
  });
});
