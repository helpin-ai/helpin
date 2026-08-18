import Konva from 'konva';
import {
  COVER_COLOR,
  HIGHLIGHT_OPACITY,
  type AnnotationShape,
  type AnnotationState,
  type CalloutShape,
} from './annotationTypes';

const CALLOUT_PADDING = 12;
const CALLOUT_CORNER = 8;
const FONT_FAMILY = 'Inter, system-ui, sans-serif';

function calloutTailPoints(shape: CalloutShape): number[] {
  const centerY = shape.y + shape.height / 2;
  const anchorX = Math.max(shape.x, Math.min(shape.tailX, shape.x + shape.width));
  const anchorY = shape.tailY < centerY ? shape.y : shape.y + shape.height;
  const spread = Math.min(18, shape.width / 3);
  return [
    anchorX - spread - shape.x,
    anchorY - shape.y,
    shape.tailX - shape.x,
    shape.tailY - shape.y,
    anchorX + spread - shape.x,
    anchorY - shape.y,
  ];
}

/**
 * Builds the konva node for one shape imperatively. This mirrors `shapes.tsx`; the two are kept
 * in sync deliberately rather than shared, because the interactive renderer carries event
 * handlers and selection concerns the export path must not have.
 */
function buildNode(shape: AnnotationShape): Konva.Shape | Konva.Group | null {
  switch (shape.type) {
    case 'arrow':
      return new Konva.Arrow({
        points: shape.points,
        stroke: shape.color,
        fill: shape.color,
        strokeWidth: shape.strokeWidth,
        pointerLength: shape.strokeWidth * 3,
        pointerWidth: shape.strokeWidth * 3,
        lineCap: 'round',
        lineJoin: 'round',
      });

    case 'rect':
      return new Konva.Rect({
        x: shape.x,
        y: shape.y,
        width: shape.width,
        height: shape.height,
        rotation: shape.rotation,
        stroke: shape.color,
        strokeWidth: shape.strokeWidth,
        cornerRadius: 2,
      });

    case 'ellipse':
      return new Konva.Ellipse({
        x: shape.x,
        y: shape.y,
        radiusX: shape.radiusX,
        radiusY: shape.radiusY,
        rotation: shape.rotation,
        stroke: shape.color,
        strokeWidth: shape.strokeWidth,
      });

    case 'text':
      return new Konva.Text({
        x: shape.x,
        y: shape.y,
        text: shape.text,
        width: shape.width,
        fill: shape.color,
        fontSize: shape.fontSize,
        fontStyle: 'bold',
        rotation: shape.rotation,
        fontFamily: FONT_FAMILY,
      });

    case 'callout': {
      const group = new Konva.Group({ x: shape.x, y: shape.y });
      group.add(
        new Konva.Line({
          points: calloutTailPoints(shape),
          closed: true,
          fill: shape.color,
          stroke: shape.color,
          strokeWidth: 1,
        }),
      );
      group.add(
        new Konva.Rect({
          x: 0,
          y: 0,
          width: shape.width,
          height: shape.height,
          fill: shape.color,
          cornerRadius: CALLOUT_CORNER,
        }),
      );
      group.add(
        new Konva.Text({
          x: CALLOUT_PADDING,
          y: CALLOUT_PADDING,
          width: shape.width - CALLOUT_PADDING * 2,
          height: shape.height - CALLOUT_PADDING * 2,
          text: shape.text,
          fill: '#ffffff',
          fontSize: shape.fontSize,
          fontStyle: 'bold',
          fontFamily: FONT_FAMILY,
          wrap: 'word',
        }),
      );
      return group;
    }

    case 'highlight':
      return new Konva.Rect({
        x: shape.x,
        y: shape.y,
        width: shape.width,
        height: shape.height,
        fill: shape.color,
        opacity: HIGHLIGHT_OPACITY,
      });

    case 'cover':
      return new Konva.Rect({
        x: shape.x,
        y: shape.y,
        width: shape.width,
        height: shape.height,
        fill: shape.color || COVER_COLOR,
        opacity: 1,
      });

    case 'freehand':
      return new Konva.Line({
        points: shape.points,
        stroke: shape.color,
        strokeWidth: shape.strokeWidth,
        lineCap: 'round',
        lineJoin: 'round',
        tension: 0.3,
      });

    default:
      return null;
  }
}

function decodeImage(src: string, crossOrigin?: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new window.Image();
    if (crossOrigin) image.crossOrigin = crossOrigin;
    image.onload = () => resolve(image);
    image.onerror = () => reject(new Error(`image load failed: ${src.slice(0, 200)}`));
    image.src = src;
  });
}

/** Appends a nonce so a CORS load never reuses a cache entry stored without CORS headers. */
function withCacheBuster(src: string): string {
  if (src.startsWith('data:') || src.startsWith('blob:')) return src;
  const separator = src.includes('?') ? '&' : '?';
  return `${src}${separator}__cors=1`;
}

/**
 * Loads an image for canvas use, which requires it to be readable without tainting.
 *
 * Fetching to a blob first is deliberate. Loading the same URL directly with
 * `crossOrigin="anonymous"` after the page has already displayed it without CORS hits a
 * long-standing browser behaviour: the cached response has no `Access-Control-Allow-Origin`,
 * so the CORS load fails even though the server would happily supply the header. A blob URL is
 * same-origin by definition, so it sidesteps the problem entirely.
 *
 * The direct load remains as a fallback for hosts that allow `<img>` but not `fetch`.
 */
export async function loadAnnotationImage(src: string): Promise<HTMLImageElement> {
  try {
    const response = await fetch(src, { mode: 'cors', credentials: 'omit', cache: 'reload' });
    if (response.ok) {
      const objectUrl = URL.createObjectURL(await response.blob());
      try {
        return await decodeImage(objectUrl);
      } finally {
        // Safe once decoding resolved — the element keeps its own decoded copy.
        URL.revokeObjectURL(objectUrl);
      }
    }
  } catch {
    // Fall through to the direct load below.
  }

  try {
    return await decodeImage(withCacheBuster(src), 'anonymous');
  } catch {
    throw new Error(
      `Could not load the image for annotation (${src.slice(0, 120)}). It may be hosted somewhere that does not allow cross-origin reads.`,
    );
  }
}

/**
 * Flattens annotation state onto the source image and returns a PNG blob.
 *
 * The offscreen stage is built at the image's NATURAL size, so the export always matches source
 * resolution regardless of the size the annotation was authored at. Rendering at display size
 * is the classic way flattening annotators produce blurry screenshots.
 */
export async function renderAnnotations(
  image: HTMLImageElement,
  state: AnnotationState,
): Promise<Blob> {
  const width = image.naturalWidth || state.baseWidth;
  const height = image.naturalHeight || state.baseHeight;

  // Coordinates were authored against baseWidth/baseHeight. If the source image is served at a
  // different size than when the annotation was made, scale rather than misplace the shapes.
  const scaleX = state.baseWidth > 0 ? width / state.baseWidth : 1;
  const scaleY = state.baseHeight > 0 ? height / state.baseHeight : 1;

  const container = document.createElement('div');
  const stage = new Konva.Stage({ container, width, height });

  try {
    const layer = new Konva.Layer({ scaleX, scaleY });
    layer.add(new Konva.Image({ image, x: 0, y: 0, width: state.baseWidth, height: state.baseHeight }));

    for (const shape of state.shapes) {
      const node = buildNode(shape);
      if (node) layer.add(node);
    }

    stage.add(layer);
    layer.draw();

    const blob = await new Promise<Blob | null>((resolve) => {
      stage.toCanvas().toBlob((result) => resolve(result), 'image/png');
    });
    if (!blob) throw new Error('Could not render the annotated image');
    return blob;
  } finally {
    stage.destroy();
  }
}

/** Convenience wrapper: load the source, flatten, and hand back a File ready for upload. */
export async function renderAnnotationsToFile(
  sourceUrl: string,
  state: AnnotationState,
  fileName: string,
): Promise<File> {
  const image = await loadAnnotationImage(sourceUrl);
  const blob = await renderAnnotations(image, state);
  return new File([blob], fileName, { type: 'image/png' });
}
