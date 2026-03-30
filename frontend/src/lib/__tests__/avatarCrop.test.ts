import { describe, expect, it } from 'vitest';
import {
  AVATAR_EDITOR_VIEWPORT_SIZE,
  clampAvatarTransform,
  getAvatarBaseScale,
  getAvatarCropGeometry,
  getAvatarOffsetBounds,
} from '@/lib/avatarCrop';

describe('avatar crop helpers', () => {
  it('scales images to fully cover the crop viewport', () => {
    expect(getAvatarBaseScale(800, 400, AVATAR_EDITOR_VIEWPORT_SIZE)).toBeCloseTo(0.8);
    expect(getAvatarBaseScale(400, 800, AVATAR_EDITOR_VIEWPORT_SIZE)).toBeCloseTo(0.8);
  });

  it('calculates drag bounds from zoomed display size', () => {
    const bounds = getAvatarOffsetBounds(800, 400, 1, AVATAR_EDITOR_VIEWPORT_SIZE);
    expect(bounds.maxOffsetX).toBeCloseTo(160);
    expect(bounds.maxOffsetY).toBeCloseTo(0);
  });

  it('clamps drag offsets so the crop area always stays covered', () => {
    const clamped = clampAvatarTransform(800, 400, { zoom: 1, offsetX: 999, offsetY: 50 }, AVATAR_EDITOR_VIEWPORT_SIZE);
    expect(clamped.offsetX).toBeCloseTo(160);
    expect(clamped.offsetY).toBe(0);
  });

  it('converts the viewport crop back to source image coordinates', () => {
    const geometry = getAvatarCropGeometry(
      800,
      400,
      { zoom: 1, offsetX: 80, offsetY: 0 },
      AVATAR_EDITOR_VIEWPORT_SIZE,
    );

    expect(geometry.sourceX).toBeCloseTo(100);
    expect(geometry.sourceY).toBeCloseTo(0);
    expect(geometry.sourceWidth).toBeCloseTo(400);
    expect(geometry.sourceHeight).toBeCloseTo(400);
  });
});
