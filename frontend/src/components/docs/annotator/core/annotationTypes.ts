// Annotation state schema. Owned by Helpin — deliberately not a third-party format, because
// this state is persisted inside document HTML and therefore lives in doc version history
// permanently.
//
// All coordinates are in SOURCE-IMAGE PIXEL SPACE, never display space. That is what makes a
// re-edit correct regardless of the dialog size the annotation was authored at.
//
// This file must not import anything Helpin-specific — see the sibling `integration/` folder.

export const ANNOTATION_STATE_VERSION = 1;

export type AnnotationShapeType =
  | 'arrow'
  | 'rect'
  | 'ellipse'
  | 'text'
  | 'callout'
  | 'highlight'
  | 'cover'
  | 'freehand';

interface ShapeBase {
  id: string;
  type: AnnotationShapeType;
}

export interface ArrowShape extends ShapeBase {
  type: 'arrow';
  /** [x1, y1, x2, y2] — the tip is the second point. */
  points: [number, number, number, number];
  color: string;
  strokeWidth: number;
}

export interface RectShape extends ShapeBase {
  type: 'rect';
  x: number;
  y: number;
  width: number;
  height: number;
  color: string;
  strokeWidth: number;
  rotation: number;
}

export interface EllipseShape extends ShapeBase {
  type: 'ellipse';
  x: number;
  y: number;
  radiusX: number;
  radiusY: number;
  color: string;
  strokeWidth: number;
  rotation: number;
}

export interface TextShape extends ShapeBase {
  type: 'text';
  x: number;
  y: number;
  text: string;
  color: string;
  fontSize: number;
  rotation: number;
}

export interface CalloutShape extends ShapeBase {
  type: 'callout';
  x: number;
  y: number;
  width: number;
  height: number;
  text: string;
  color: string;
  /** Tail tip, in source pixels — what the callout points at. */
  tailX: number;
  tailY: number;
}

export interface HighlightShape extends ShapeBase {
  type: 'highlight';
  x: number;
  y: number;
  width: number;
  height: number;
  color: string;
}

/**
 * Opaque redaction. Deliberately not a blur or pixelate: those are reversible in principle,
 * an opaque fill genuinely destroys the pixels in the flattened output.
 */
export interface CoverShape extends ShapeBase {
  type: 'cover';
  x: number;
  y: number;
  width: number;
  height: number;
  color: string;
}

export interface FreehandShape extends ShapeBase {
  type: 'freehand';
  /** Flat [x0, y0, x1, y1, …] as konva's Line expects. */
  points: number[];
  color: string;
  strokeWidth: number;
}

export type AnnotationShape =
  | ArrowShape
  | RectShape
  | EllipseShape
  | TextShape
  | CalloutShape
  | HighlightShape
  | CoverShape
  | FreehandShape;

export interface AnnotationState {
  version: number;
  /** Natural size of the source image the coordinates are expressed against. */
  baseWidth: number;
  baseHeight: number;
  shapes: AnnotationShape[];
}

export const DEFAULT_ANNOTATION_COLOR = '#ef4444';
export const DEFAULT_STROKE_WIDTH = 4;
export const DEFAULT_FONT_SIZE = 24;
export const COVER_COLOR = '#111827';
export const HIGHLIGHT_OPACITY = 0.35;

export function emptyAnnotationState(baseWidth: number, baseHeight: number): AnnotationState {
  return { version: ANNOTATION_STATE_VERSION, baseWidth, baseHeight, shapes: [] };
}

export function hasCoverShape(state: AnnotationState | null | undefined): boolean {
  return Boolean(state?.shapes.some((shape) => shape.type === 'cover'));
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value);
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0;
}

function numberOr(value: unknown, fallback: number): number {
  return isFiniteNumber(value) ? value : fallback;
}

/**
 * Validates one shape. Returns null for anything unrecognised or malformed — an unknown shape
 * type is dropped, never thrown on, so a state written by a newer client still opens here.
 */
function parseShape(raw: unknown): AnnotationShape | null {
  if (!raw || typeof raw !== 'object') return null;
  const shape = raw as Record<string, unknown>;
  const id = isNonEmptyString(shape.id) ? shape.id : null;
  if (!id) return null;
  const color = isNonEmptyString(shape.color) ? shape.color : DEFAULT_ANNOTATION_COLOR;

  switch (shape.type) {
    case 'arrow': {
      const points = shape.points;
      if (!Array.isArray(points) || points.length !== 4 || !points.every(isFiniteNumber)) return null;
      return {
        id,
        type: 'arrow',
        points: [points[0], points[1], points[2], points[3]],
        color,
        strokeWidth: numberOr(shape.strokeWidth, DEFAULT_STROKE_WIDTH),
      };
    }
    case 'rect':
    case 'highlight':
    case 'cover': {
      if (!isFiniteNumber(shape.x) || !isFiniteNumber(shape.y)) return null;
      if (!isFiniteNumber(shape.width) || !isFiniteNumber(shape.height)) return null;
      const box = { id, x: shape.x, y: shape.y, width: shape.width, height: shape.height, color };
      if (shape.type === 'rect') {
        return {
          ...box,
          type: 'rect',
          strokeWidth: numberOr(shape.strokeWidth, DEFAULT_STROKE_WIDTH),
          rotation: numberOr(shape.rotation, 0),
        };
      }
      return { ...box, type: shape.type };
    }
    case 'ellipse': {
      if (!isFiniteNumber(shape.x) || !isFiniteNumber(shape.y)) return null;
      if (!isFiniteNumber(shape.radiusX) || !isFiniteNumber(shape.radiusY)) return null;
      return {
        id,
        type: 'ellipse',
        x: shape.x,
        y: shape.y,
        radiusX: shape.radiusX,
        radiusY: shape.radiusY,
        color,
        strokeWidth: numberOr(shape.strokeWidth, DEFAULT_STROKE_WIDTH),
        rotation: numberOr(shape.rotation, 0),
      };
    }
    case 'text': {
      if (!isFiniteNumber(shape.x) || !isFiniteNumber(shape.y)) return null;
      if (typeof shape.text !== 'string') return null;
      return {
        id,
        type: 'text',
        x: shape.x,
        y: shape.y,
        text: shape.text,
        color,
        fontSize: numberOr(shape.fontSize, DEFAULT_FONT_SIZE),
        rotation: numberOr(shape.rotation, 0),
      };
    }
    case 'callout': {
      if (!isFiniteNumber(shape.x) || !isFiniteNumber(shape.y)) return null;
      if (!isFiniteNumber(shape.width) || !isFiniteNumber(shape.height)) return null;
      if (typeof shape.text !== 'string') return null;
      return {
        id,
        type: 'callout',
        x: shape.x,
        y: shape.y,
        width: shape.width,
        height: shape.height,
        text: shape.text,
        color,
        tailX: numberOr(shape.tailX, shape.x),
        tailY: numberOr(shape.tailY, shape.y + shape.height + 40),
      };
    }
    case 'freehand': {
      const points = shape.points;
      if (!Array.isArray(points) || points.length < 4 || !points.every(isFiniteNumber)) return null;
      return {
        id,
        type: 'freehand',
        points: [...points],
        color,
        strokeWidth: numberOr(shape.strokeWidth, DEFAULT_STROKE_WIDTH),
      };
    }
    default:
      return null;
  }
}

/**
 * Parses persisted annotation state. Accepts either the object itself or its JSON string form.
 * Returns null rather than throwing on anything malformed — this runs during document load, so
 * one bad attribute must never take the editor down.
 */
export function parseAnnotationState(raw: unknown): AnnotationState | null {
  if (raw == null) return null;

  let value = raw;
  if (typeof value === 'string') {
    const trimmed = value.trim();
    if (!trimmed) return null;
    try {
      value = JSON.parse(trimmed);
    } catch {
      return null;
    }
  }

  if (!value || typeof value !== 'object') return null;
  const state = value as Record<string, unknown>;
  if (!isFiniteNumber(state.baseWidth) || !isFiniteNumber(state.baseHeight)) return null;
  if (state.baseWidth <= 0 || state.baseHeight <= 0) return null;
  if (!Array.isArray(state.shapes)) return null;

  const shapes = state.shapes
    .map(parseShape)
    .filter((shape): shape is AnnotationShape => shape !== null);

  return {
    version: isFiniteNumber(state.version) ? state.version : ANNOTATION_STATE_VERSION,
    baseWidth: state.baseWidth,
    baseHeight: state.baseHeight,
    shapes,
  };
}

/**
 * Display scale for the authoring stage.
 *
 * Never returns more than 1: an upscaled source would look soft while authoring and misrepresent
 * the export, which is always rendered at natural size.
 */
export function fitScale(
  natural: { width: number; height: number },
  containerWidth: number,
  maxHeight: number,
): number {
  if (!natural.width || !natural.height || !containerWidth) return 1;
  return Math.min(containerWidth / natural.width, maxHeight / natural.height, 1);
}

/** Stable id generator. Not security-sensitive — it only needs to be unique within one state. */
export function createShapeId(): string {
  return `sh-${Math.random().toString(36).slice(2, 10)}`;
}

/** Bounding box of a shape in source-pixel space, used for selection and transform handles. */
export function shapeBounds(shape: AnnotationShape): { x: number; y: number; width: number; height: number } {
  switch (shape.type) {
    case 'arrow': {
      const [x1, y1, x2, y2] = shape.points;
      return { x: Math.min(x1, x2), y: Math.min(y1, y2), width: Math.abs(x2 - x1), height: Math.abs(y2 - y1) };
    }
    case 'ellipse':
      return {
        x: shape.x - shape.radiusX,
        y: shape.y - shape.radiusY,
        width: shape.radiusX * 2,
        height: shape.radiusY * 2,
      };
    case 'text':
      return { x: shape.x, y: shape.y, width: shape.text.length * shape.fontSize * 0.6, height: shape.fontSize * 1.2 };
    case 'freehand': {
      const xs = shape.points.filter((_, index) => index % 2 === 0);
      const ys = shape.points.filter((_, index) => index % 2 === 1);
      const minX = Math.min(...xs);
      const minY = Math.min(...ys);
      return { x: minX, y: minY, width: Math.max(...xs) - minX, height: Math.max(...ys) - minY };
    }
    default:
      return { x: shape.x, y: shape.y, width: shape.width, height: shape.height };
  }
}
