import type Phaser from "phaser";

const LINEAR_FILTER_MODE: Phaser.Textures.FilterMode = 0;

export type WorldTextureSamplingMode =
  | "mipmapped-minification"
  | "linear-minification"
  | "canvas-linear"
  | "unavailable";

type TextureSourceLike = {
  width: number;
  height: number;
  glTexture: Phaser.Renderer.WebGL.Wrappers.WebGLTextureWrapper | null;
};

type TextureLike = {
  source?: TextureSourceLike[];
  setFilter?: (filterMode: Phaser.Textures.FilterMode) => unknown;
};

function isPowerOfTwo(value: number): boolean {
  return value > 0 && (value & (value - 1)) === 0;
}

function supportsNonPowerOfTwoMipmaps(gl: WebGLRenderingContext): boolean {
  // WebGL 2 permits mipmaps for NPOT textures. texStorage2D is a reliable
  // capability check that is also straightforward to model in focused tests.
  return "texStorage2D" in gl && typeof gl.texStorage2D === "function";
}

/**
 * Refresh minification sampling after a world texture is created or redrawn.
 *
 * CaptureQuest deliberately keeps Phaser's global pixel-art mode so sprites,
 * UI, and magnified map pixels remain crisp. World surfaces are different:
 * the camera routinely displays several source texels inside one screen pixel.
 * Their minifier therefore uses trilinear mipmaps while magnification remains
 * nearest-neighbor. WebGL 1 cannot mipmap NPOT textures, so those surfaces use
 * bounded linear minification rather than unstable nearest sampling.
 */
export function refreshWorldTextureSampling(
  scene: Phaser.Scene,
  texture: TextureLike | null | undefined,
): WorldTextureSamplingMode {
  if (!texture) return "unavailable";

  const renderer = scene.game?.renderer as
    | Phaser.Renderer.WebGL.WebGLRenderer
    | Phaser.Renderer.Canvas.CanvasRenderer
    | undefined;
  if (!renderer || !("gl" in renderer) || !renderer.gl) {
    texture.setFilter?.(LINEAR_FILTER_MODE);
    return "canvas-linear";
  }

  const source = texture.source?.[0];
  const wrapper = source?.glTexture;
  const webGLTexture = wrapper?.webGLTexture;
  if (!source || !wrapper || !webGLTexture) return "unavailable";

  const gl = renderer.gl;
  const canGenerateMipmaps =
    (isPowerOfTwo(source.width) && isPowerOfTwo(source.height)) ||
    supportsNonPowerOfTwoMipmaps(gl);
  const minFilter = canGenerateMipmaps
    ? gl.LINEAR_MIPMAP_LINEAR
    : gl.LINEAR;
  const magFilter = gl.NEAREST;
  const previousActiveTexture = gl.getParameter(gl.ACTIVE_TEXTURE) as number;

  gl.activeTexture(gl.TEXTURE0);
  const previousTexture = gl.getParameter(
    gl.TEXTURE_BINDING_2D,
  ) as WebGLTexture | null;
  gl.bindTexture(gl.TEXTURE_2D, webGLTexture);
  if (canGenerateMipmaps) gl.generateMipmap(gl.TEXTURE_2D);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, minFilter);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, magFilter);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
  gl.bindTexture(gl.TEXTURE_2D, previousTexture);
  gl.activeTexture(previousActiveTexture);

  // Phaser owns context restoration, so mirror the parameters onto its wrapper
  // instead of changing only the ephemeral WebGL object.
  wrapper.minFilter = minFilter;
  wrapper.magFilter = magFilter;
  wrapper.wrapS = gl.CLAMP_TO_EDGE;
  wrapper.wrapT = gl.CLAMP_TO_EDGE;

  return canGenerateMipmaps
    ? "mipmapped-minification"
    : "linear-minification";
}
