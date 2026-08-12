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
  return [anchorX - spread, anchorY, shape.tailX, shape.tailY, anchorX + spread, anchorY];
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
        fill: shape.color,
        fontSize: shape.fontSize,
        fontStyle: 'bold',
        rotation: shape.rotation,
        fontFamily: FONT_FAMILY,
      });

    case 'callout': {
      const group = new Konva.Group();
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
          x: shape.x,
          y: shape.y,
          width: shape.width,
          height: shape.height,
          fill: shape.color,
          cornerRadius: CALLOUT_CORNER,
        }),
      );
      group.add(
        new Konva.Text({
          x: shape.x + CALLOUT_PADDING,
          y: shape.y + CALLOUT_PADDING,
          width: shape.width - CALLOUT_PADDING * 2,
          height: shape.height - CALLOUT_PADDING * 2,
          text: shape.text,
          fill: '#ffffff',
          fontSize: 16,
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

/** Loads an image element for rendering. `crossOrigin` is required or the canvas taints. */
export function loadAnnotationImage(src: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new window.Image();
    image.crossOrigin = 'anonymous';
    image.onload = () => resolve(image);
    // The load is anonymous-CORS, so this fires for a genuinely missing image AND for a host
    // that serves the image without CORS headers. Say so — the second case is otherwise a
    // baffling failure on an image the user can plainly see on the page.
    image.onerror = () =>
      reject(
        new Error(
          'Could not load the image for annotation. It may be hosted somewhere that does not allow cross-origin reads.',
        ),
      );
    image.src = src;
  });
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
