export const AVATAR_EDITOR_VIEWPORT_SIZE = 320;
export const AVATAR_OUTPUT_SIZE = 512;

export type AvatarTransform = {
  zoom: number;
  offsetX: number;
  offsetY: number;
};

export type AvatarGeometry = {
  baseScale: number;
  effectiveScale: number;
  displayWidth: number;
  displayHeight: number;
  maxOffsetX: number;
  maxOffsetY: number;
  imageLeft: number;
  imageTop: number;
  sourceX: number;
  sourceY: number;
  sourceWidth: number;
  sourceHeight: number;
};

export function getAvatarBaseScale(imageWidth: number, imageHeight: number, viewportSize = AVATAR_EDITOR_VIEWPORT_SIZE): number {
  if (imageWidth <= 0 || imageHeight <= 0) {
    return 1;
  }
  return Math.max(viewportSize / imageWidth, viewportSize / imageHeight);
}

export function getAvatarOffsetBounds(
  imageWidth: number,
  imageHeight: number,
  zoom: number,
  viewportSize = AVATAR_EDITOR_VIEWPORT_SIZE,
) {
  const baseScale = getAvatarBaseScale(imageWidth, imageHeight, viewportSize);
  const effectiveScale = baseScale * zoom;
  const displayWidth = imageWidth * effectiveScale;
  const displayHeight = imageHeight * effectiveScale;

  return {
    maxOffsetX: Math.max((displayWidth - viewportSize) / 2, 0),
    maxOffsetY: Math.max((displayHeight - viewportSize) / 2, 0),
  };
}

export function clampAvatarTransform(
  imageWidth: number,
  imageHeight: number,
  transform: AvatarTransform,
  viewportSize = AVATAR_EDITOR_VIEWPORT_SIZE,
): AvatarTransform {
  const { maxOffsetX, maxOffsetY } = getAvatarOffsetBounds(imageWidth, imageHeight, transform.zoom, viewportSize);

  return {
    zoom: transform.zoom,
    offsetX: Math.min(Math.max(transform.offsetX, -maxOffsetX), maxOffsetX),
    offsetY: Math.min(Math.max(transform.offsetY, -maxOffsetY), maxOffsetY),
  };
}

export function getAvatarCropGeometry(
  imageWidth: number,
  imageHeight: number,
  transform: AvatarTransform,
  viewportSize = AVATAR_EDITOR_VIEWPORT_SIZE,
): AvatarGeometry {
  const clamped = clampAvatarTransform(imageWidth, imageHeight, transform, viewportSize);
  const baseScale = getAvatarBaseScale(imageWidth, imageHeight, viewportSize);
  const effectiveScale = baseScale * clamped.zoom;
  const displayWidth = imageWidth * effectiveScale;
  const displayHeight = imageHeight * effectiveScale;
  const maxOffsetX = Math.max((displayWidth - viewportSize) / 2, 0);
  const maxOffsetY = Math.max((displayHeight - viewportSize) / 2, 0);
  const imageLeft = ((viewportSize - displayWidth) / 2) + clamped.offsetX;
  const imageTop = ((viewportSize - displayHeight) / 2) + clamped.offsetY;
  const sourceX = Math.max((0 - imageLeft) / effectiveScale, 0);
  const sourceY = Math.max((0 - imageTop) / effectiveScale, 0);
  const sourceWidth = Math.min(viewportSize / effectiveScale, imageWidth - sourceX);
  const sourceHeight = Math.min(viewportSize / effectiveScale, imageHeight - sourceY);

  return {
    baseScale,
    effectiveScale,
    displayWidth,
    displayHeight,
    maxOffsetX,
    maxOffsetY,
    imageLeft,
    imageTop,
    sourceX,
    sourceY,
    sourceWidth,
    sourceHeight,
  };
}

export function drawAvatarCropToCanvas(
  image: CanvasImageSource,
  imageWidth: number,
  imageHeight: number,
  transform: AvatarTransform,
  canvas: HTMLCanvasElement,
  viewportSize = AVATAR_EDITOR_VIEWPORT_SIZE,
  outputSize = AVATAR_OUTPUT_SIZE,
) {
  const geometry = getAvatarCropGeometry(imageWidth, imageHeight, transform, viewportSize);
  const context = canvas.getContext('2d');

  if (!context) {
    throw new Error('Canvas context is unavailable');
  }

  canvas.width = outputSize;
  canvas.height = outputSize;
  context.clearRect(0, 0, outputSize, outputSize);
  context.imageSmoothingEnabled = true;
  context.imageSmoothingQuality = 'high';

  // Draw the visible square crop at a fixed export resolution so uploaded avatars stay crisp.
  context.drawImage(
    image,
    geometry.sourceX,
    geometry.sourceY,
    geometry.sourceWidth,
    geometry.sourceHeight,
    0,
    0,
    outputSize,
    outputSize,
  );
}

export async function canvasToFile(
  canvas: HTMLCanvasElement,
  fileName: string,
  type = 'image/png',
): Promise<File> {
  const blob = await new Promise<Blob>((resolve, reject) => {
    canvas.toBlob((value) => {
      if (value) {
        resolve(value);
        return;
      }
      reject(new Error('Failed to create cropped avatar image'));
    }, type);
  });

  return new File([blob], fileName, { type });
}
