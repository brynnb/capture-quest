import { describe, expect, test, vi } from "vitest";

import { refreshWorldTextureSampling } from "./worldTextureSampling";

const GL = {
  ACTIVE_TEXTURE: 0x84e0,
  TEXTURE0: 0x84c0,
  TEXTURE_2D: 0x0de1,
  TEXTURE_BINDING_2D: 0x8069,
  TEXTURE_MIN_FILTER: 0x2801,
  TEXTURE_MAG_FILTER: 0x2800,
  TEXTURE_WRAP_S: 0x2802,
  TEXTURE_WRAP_T: 0x2803,
  LINEAR: 0x2601,
  NEAREST: 0x2600,
  LINEAR_MIPMAP_LINEAR: 0x2703,
  CLAMP_TO_EDGE: 0x812f,
};

function harness(width: number, height: number, webGL2 = false) {
  const previousTexture = { id: "previous" };
  const worldTexture = { id: "world" };
  const gl = {
    ...GL,
    getParameter: vi.fn((parameter: number) =>
      parameter === GL.ACTIVE_TEXTURE ? GL.TEXTURE0 + 2 : previousTexture,
    ),
    activeTexture: vi.fn(),
    bindTexture: vi.fn(),
    generateMipmap: vi.fn(),
    texParameteri: vi.fn(),
    ...(webGL2 ? { texStorage2D: vi.fn() } : {}),
  };
  const wrapper = {
    webGLTexture: worldTexture,
    minFilter: GL.NEAREST,
    magFilter: GL.NEAREST,
    wrapS: GL.CLAMP_TO_EDGE,
    wrapT: GL.CLAMP_TO_EDGE,
  };
  const texture = { source: [{ width, height, glTexture: wrapper }] };
  const scene = { game: { renderer: { gl } } };
  return { gl, wrapper, texture, scene, previousTexture, worldTexture };
}

describe("world texture minification sampling", () => {
  test("generates trilinear mipmaps for power-of-two WebGL 1 surfaces", () => {
    const { gl, wrapper, texture, scene, previousTexture, worldTexture } =
      harness(256, 256);

    expect(
      refreshWorldTextureSampling(scene as never, texture as never),
    ).toBe("mipmapped-minification");
    expect(gl.generateMipmap).toHaveBeenCalledWith(GL.TEXTURE_2D);
    expect(gl.bindTexture).toHaveBeenNthCalledWith(
      1,
      GL.TEXTURE_2D,
      worldTexture,
    );
    expect(gl.bindTexture).toHaveBeenLastCalledWith(
      GL.TEXTURE_2D,
      previousTexture,
    );
    expect(wrapper).toMatchObject({
      minFilter: GL.LINEAR_MIPMAP_LINEAR,
      magFilter: GL.NEAREST,
    });
  });

  test("uses linear minification for NPOT WebGL 1 surfaces", () => {
    const { gl, wrapper, texture, scene } = harness(1040, 1040);

    expect(
      refreshWorldTextureSampling(scene as never, texture as never),
    ).toBe("linear-minification");
    expect(gl.generateMipmap).not.toHaveBeenCalled();
    expect(wrapper).toMatchObject({
      minFilter: GL.LINEAR,
      magFilter: GL.NEAREST,
    });
  });

  test("generates mipmaps for NPOT WebGL 2 surfaces", () => {
    const { gl, wrapper, texture, scene } = harness(1040, 1040, true);

    expect(
      refreshWorldTextureSampling(scene as never, texture as never),
    ).toBe("mipmapped-minification");
    expect(gl.generateMipmap).toHaveBeenCalledOnce();
    expect(wrapper.minFilter).toBe(GL.LINEAR_MIPMAP_LINEAR);
  });

  test("enables linear canvas sampling without touching WebGL", () => {
    const setFilter = vi.fn();
    const scene = { game: { renderer: {} } };

    expect(
      refreshWorldTextureSampling(scene as never, { setFilter }),
    ).toBe("canvas-linear");
    expect(setFilter).toHaveBeenCalledWith(0);
  });
});
