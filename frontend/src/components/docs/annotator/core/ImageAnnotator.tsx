import { forwardRef, useCallback, useEffect, useImperativeHandle, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { Image as KonvaImage, Layer, Stage, Transformer, Circle } from 'react-konva';
import type Konva from 'konva';
import {
  COVER_COLOR,
  createShapeId,
  fitScale,
  type AnnotationShape,
  type ArrowShape,
  type CalloutShape,
  type FreehandShape,
} from './annotationTypes';
import { AnnotationShapeNode } from './shapes';
import { loadAnnotationImage } from './renderAnnotations';

export type AnnotationTool =
  | 'select'
  | 'arrow'
  | 'rect'
  | 'ellipse'
  | 'text'
  | 'callout'
  | 'highlight'
  | 'cover'
  | 'freehand';

/** Shapes whose konva node carries its own coordinates; the rest report a drag delta. */
const BOX_TOOLS = new Set<AnnotationShape['type']>(['rect', 'ellipse', 'text', 'highlight', 'cover']);
/** Shapes the Transformer can resize. Callouts are drag-only — their children are absolute. */
const TRANSFORMABLE = new Set<AnnotationShape['type']>(['rect', 'ellipse', 'text', 'highlight', 'cover']);

const MIN_DRAG_PIXELS = 6;
const DEFAULT_CALLOUT_TEXT = 'Add a note';

export interface ImageAnnotatorProps {
  imageUrl: string;
  tool: AnnotationTool;
  color: string;
  strokeWidth: number;
  fontSize: number;
  shapes: AnnotationShape[];
  /** Pushes a new undo entry. */
  onCommit: (next: AnnotationShape[]) => void;
  selectedId: string | null;
  onSelectedIdChange: (id: string | null) => void;
  /** Fired once the source image is measured — the caller needs it for baseWidth/baseHeight. */
  onImageLoaded?: (size: { width: number; height: number }) => void;
  onError?: (message: string) => void;
  /** Tool resets to 'select' after a one-shot creation so the user is not stuck drawing. */
  onToolConsumed?: () => void;
  maxHeight?: number;
}

export interface ImageAnnotatorHandle {
  /** Includes text still being edited in the DOM overlay, which is not yet in undo history. */
  getShapesForSave: () => AnnotationShape[];
}

interface EditingText {
  id: string;
  value: string;
  /** Display-space position of the editor overlay. */
  left: number;
  top: number;
  width: number;
  fontSize: number;
}

function applyTextEdit(shapes: AnnotationShape[], editing: EditingText | null): AnnotationShape[] {
  if (!editing) return shapes;
  const target = shapes.find((shape) => shape.id === editing.id);
  if (!target || (target.type !== 'text' && target.type !== 'callout')) return shapes;

  const value = editing.value.trim();
  if (!value && target.type === 'text') {
    return shapes.filter((shape) => shape.id !== editing.id);
  }
  return shapes.map((shape) =>
    shape.id === editing.id
      ? ({ ...shape, text: value || DEFAULT_CALLOUT_TEXT } as AnnotationShape)
      : shape,
  );
}

export const ImageAnnotator = forwardRef<ImageAnnotatorHandle, ImageAnnotatorProps>(function ImageAnnotator({
  imageUrl,
  tool,
  color,
  strokeWidth,
  fontSize,
  shapes,
  onCommit,
  selectedId,
  onSelectedIdChange,
  onImageLoaded,
  onError,
  onToolConsumed,
  maxHeight = 520,
}, ref) {
  const containerRef = useRef<HTMLDivElement>(null);
  const stageRef = useRef<Konva.Stage>(null);
  const transformerRef = useRef<Konva.Transformer>(null);
  const layerRef = useRef<Konva.Layer>(null);

  // Keyed by url so a source change invalidates the previous image without a reset effect.
  const [loaded, setLoaded] = useState<{ url: string; image: HTMLImageElement } | null>(null);
  const [containerWidth, setContainerWidth] = useState(0);
  const [draft, setDraft] = useState<AnnotationShape | null>(null);
  const [editingText, setEditingText] = useState<EditingText | null>(null);
  const drawingRef = useRef(false);
  const originRef = useRef<{ x: number; y: number } | null>(null);
  // Written only from pointer handlers, never during render. Lets pointerup commit the finished
  // draft without running side effects inside a state updater (React double-invokes those).
  const draftRef = useRef<AnnotationShape | null>(null);

  const image = loaded?.url === imageUrl ? loaded.image : null;

  // ── Source image ────────────────────────────────────────────────────────────
  useEffect(() => {
    let cancelled = false;
    loadAnnotationImage(imageUrl)
      .then((element) => {
        if (cancelled) return;
        setLoaded({ url: imageUrl, image: element });
        onImageLoaded?.({ width: element.naturalWidth, height: element.naturalHeight });
      })
      .catch((error: Error) => {
        if (!cancelled) onError?.(error.message);
      });
    return () => {
      cancelled = true;
    };
    // onImageLoaded/onError are callbacks from the caller; re-running on their identity would
    // reload the image on every parent render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [imageUrl]);

  useLayoutEffect(() => {
    const element = containerRef.current;
    if (!element) return;
    const observer = new ResizeObserver((entries) => {
      const width = entries[0]?.contentRect.width ?? 0;
      setContainerWidth(width);
    });
    observer.observe(element);
    setContainerWidth(element.clientWidth);
    return () => observer.disconnect();
  }, []);

  const naturalWidth = image?.naturalWidth ?? 0;
  const naturalHeight = image?.naturalHeight ?? 0;

  const scale = useMemo(
    () => fitScale({ width: naturalWidth, height: naturalHeight }, containerWidth, maxHeight),
    [containerWidth, maxHeight, naturalWidth, naturalHeight],
  );

  const stageWidth = Math.round(naturalWidth * scale);
  const stageHeight = Math.round(naturalHeight * scale);

  // ── Coordinate conversion ───────────────────────────────────────────────────
  const pointerInSourceSpace = useCallback((): { x: number; y: number } | null => {
    const stage = stageRef.current;
    const position = stage?.getPointerPosition();
    if (!position || !scale) return null;
    return { x: position.x / scale, y: position.y / scale };
  }, [scale]);

  // ── Selection / transformer ─────────────────────────────────────────────────
  const selectedShape = useMemo(
    () => shapes.find((shape) => shape.id === selectedId) ?? null,
    [shapes, selectedId],
  );

  useEffect(() => {
    const transformer = transformerRef.current;
    const layer = layerRef.current;
    if (!transformer || !layer) return;
    if (!selectedShape || !TRANSFORMABLE.has(selectedShape.type)) {
      transformer.nodes([]);
      return;
    }
    const node = layer.findOne(`#${selectedShape.id}`);
    transformer.nodes(node ? [node] : []);
  }, [selectedShape, shapes]);

  const replaceShape = useCallback(
    (id: string, updater: (shape: AnnotationShape) => AnnotationShape) => {
      onCommit(shapes.map((shape) => (shape.id === id ? updater(shape) : shape)));
    },
    [onCommit, shapes],
  );

  // ── Drag ────────────────────────────────────────────────────────────────────
  const handleDragEnd = useCallback(
    (id: string, node: Konva.Node) => {
      const shape = shapes.find((candidate) => candidate.id === id);
      if (!shape) return;
      const x = node.x();
      const y = node.y();

      if (BOX_TOOLS.has(shape.type)) {
        replaceShape(id, (current) => ({ ...current, x, y }) as AnnotationShape);
        return;
      }

      // Delta movers: fold the offset into the absolute geometry, then zero the node so the
      // next drag starts from the origin again.
      node.position({ x: 0, y: 0 });
      replaceShape(id, (current) => {
        switch (current.type) {
          case 'arrow': {
            const [x1, y1, x2, y2] = current.points;
            return { ...current, points: [x1 + x, y1 + y, x2 + x, y2 + y] } as ArrowShape;
          }
          case 'freehand':
            return {
              ...current,
              points: current.points.map((value, index) => (index % 2 === 0 ? value + x : value + y)),
            } as FreehandShape;
          case 'callout':
            return {
              ...current,
              x: current.x + x,
              y: current.y + y,
              tailX: current.tailX + x,
              tailY: current.tailY + y,
            } as CalloutShape;
          default:
            return current;
        }
      });
    },
    [replaceShape, shapes],
  );

  // ── Transform (resize / rotate) ─────────────────────────────────────────────
  const handleTransformEnd = useCallback(() => {
    const transformer = transformerRef.current;
    const node = transformer?.nodes()[0];
    if (!node || !selectedShape) return;

    const scaleX = node.scaleX();
    const scaleY = node.scaleY();
    node.scaleX(1);
    node.scaleY(1);

    replaceShape(selectedShape.id, (current) => {
      const base = { ...current, x: node.x(), y: node.y(), rotation: node.rotation() };
      switch (current.type) {
        case 'ellipse':
          return {
            ...base,
            radiusX: Math.max(2, current.radiusX * scaleX),
            radiusY: Math.max(2, current.radiusY * scaleY),
          } as AnnotationShape;
        case 'text':
          return { ...base, fontSize: Math.max(8, current.fontSize * scaleY) } as AnnotationShape;
        case 'rect':
        case 'highlight':
        case 'cover':
          return {
            ...base,
            width: Math.max(4, current.width * scaleX),
            height: Math.max(4, current.height * scaleY),
          } as AnnotationShape;
        default:
          return current;
      }
    });
  }, [replaceShape, selectedShape]);

  // ── Text editing overlay (konva has no text input) ──────────────────────────
  const openTextEditor = useCallback(
    (shape: AnnotationShape) => {
      if (shape.type !== 'text' && shape.type !== 'callout') return;
      const padding = shape.type === 'callout' ? 12 : 0;
      setEditingText({
        id: shape.id,
        value: shape.text,
        left: (shape.x + padding) * scale,
        top: (shape.y + padding) * scale,
        width: (shape.type === 'callout' ? shape.width - padding * 2 : 260) * scale,
        fontSize: (shape.type === 'callout' ? 16 : shape.fontSize) * scale,
      });
    },
    [scale],
  );

  const commitTextEditor = useCallback(() => {
    if (!editingText) return;
    const target = shapes.find((shape) => shape.id === editingText.id);
    setEditingText(null);
    if (!target) return;

    // An empty text shape is invisible and unselectable — drop it rather than stranding it.
    const next = applyTextEdit(shapes, editingText);
    if (!editingText.value.trim() && target.type === 'text') {
      onCommit(next);
      onSelectedIdChange(null);
      return;
    }
    if (target.type !== 'text' && target.type !== 'callout') return;
    onCommit(next);
  }, [editingText, onCommit, onSelectedIdChange, shapes]);

  useImperativeHandle(
    ref,
    () => ({ getShapesForSave: () => applyTextEdit(shapes, editingText) }),
    [editingText, shapes],
  );

  const handleDoubleClick = useCallback(
    (id: string) => {
      const shape = shapes.find((candidate) => candidate.id === id);
      if (shape) openTextEditor(shape);
    },
    [openTextEditor, shapes],
  );

  // ── Drawing ─────────────────────────────────────────────────────────────────
  /** Keeps the ref and the rendered draft in step. Only ever called from pointer handlers. */
  const applyDraft = useCallback((next: AnnotationShape | null) => {
    draftRef.current = next;
    setDraft(next);
  }, []);

  const handlePointerDown = useCallback(
    (event: Konva.KonvaEventObject<PointerEvent>) => {
      if (editingText) {
        commitTextEditor();
        return;
      }
      if (tool === 'select') {
        // A click on empty canvas clears the selection.
        if (event.target === event.target.getStage() || event.target.name() !== 'annotation-shape') {
          onSelectedIdChange(null);
        }
        return;
      }

      const point = pointerInSourceSpace();
      if (!point) return;
      originRef.current = point;
      drawingRef.current = true;
      onSelectedIdChange(null);

      const id = createShapeId();
      switch (tool) {
        case 'arrow':
          applyDraft({ id, type: 'arrow', points: [point.x, point.y, point.x, point.y], color, strokeWidth });
          break;
        case 'rect':
          applyDraft({ id, type: 'rect', x: point.x, y: point.y, width: 0, height: 0, color, strokeWidth, rotation: 0 });
          break;
        case 'ellipse':
          applyDraft({ id, type: 'ellipse', x: point.x, y: point.y, radiusX: 0, radiusY: 0, color, strokeWidth, rotation: 0 });
          break;
        case 'highlight':
          applyDraft({ id, type: 'highlight', x: point.x, y: point.y, width: 0, height: 0, color });
          break;
        case 'cover':
          applyDraft({ id, type: 'cover', x: point.x, y: point.y, width: 0, height: 0, color: COVER_COLOR });
          break;
        case 'freehand':
          applyDraft({ id, type: 'freehand', points: [point.x, point.y], color, strokeWidth });
          break;
        case 'callout':
          applyDraft({
            id,
            type: 'callout',
            x: point.x,
            y: point.y,
            width: 0,
            height: 0,
            text: DEFAULT_CALLOUT_TEXT,
            color,
            tailX: point.x,
            tailY: point.y,
          });
          break;
        case 'text': {
          // Text is a click, not a drag. Keep it as a draft until pointer-up before mounting
          // the textarea: mounting an auto-focused input during pointer-down lets the browser's
          // remaining click focus the canvas again, immediately blurring and deleting the empty
          // text shape.
          const shape: AnnotationShape = {
            id,
            type: 'text',
            x: point.x,
            y: point.y,
            text: '',
            color,
            fontSize,
            rotation: 0,
          };
          applyDraft(shape);
          break;
        }
      }
    },
    [
      applyDraft,
      color,
      commitTextEditor,
      editingText,
      fontSize,
      onSelectedIdChange,
      pointerInSourceSpace,
      strokeWidth,
      tool,
    ],
  );

  const handlePointerMove = useCallback(() => {
    if (!drawingRef.current) return;
    const point = pointerInSourceSpace();
    const origin = originRef.current;
    if (!point || !origin) return;

    const current = draftRef.current;
    if (!current) return;

    // Boxes are normalised to top-left + size so a drag in any direction produces valid
    // geometry rather than negative width.
    const left = Math.min(origin.x, point.x);
    const top = Math.min(origin.y, point.y);
    const boxWidth = Math.abs(point.x - origin.x);
    const boxHeight = Math.abs(point.y - origin.y);

    switch (current.type) {
      case 'arrow':
        applyDraft({ ...current, points: [origin.x, origin.y, point.x, point.y] });
        break;
      case 'freehand':
        applyDraft({ ...current, points: [...current.points, point.x, point.y] });
        break;
      case 'ellipse':
        applyDraft({
          ...current,
          x: (origin.x + point.x) / 2,
          y: (origin.y + point.y) / 2,
          radiusX: boxWidth / 2,
          radiusY: boxHeight / 2,
        });
        break;
      case 'callout':
        applyDraft({
          ...current,
          x: left,
          y: top,
          width: boxWidth,
          height: boxHeight,
          tailX: left + boxWidth / 2,
          tailY: top + boxHeight + 48,
        });
        break;
      default:
        applyDraft({ ...current, x: left, y: top, width: boxWidth, height: boxHeight } as AnnotationShape);
    }
  }, [applyDraft, pointerInSourceSpace]);

  const handlePointerUp = useCallback(() => {
    if (!drawingRef.current) return;
    drawingRef.current = false;
    originRef.current = null;

    const current = draftRef.current;
    applyDraft(null);
    if (!current || !isDrawnShapeUsable(current, scale)) return;

    onCommit([...shapes, current]);
    onSelectedIdChange(current.id);
    if (current.type === 'text' || current.type === 'callout') {
      // Let the native click finish before focusing the DOM editor. If it mounts during
      // pointer-up, the click's default focus action puts focus back on the canvas and the
      // textarea immediately blurs.
      window.requestAnimationFrame(() => openTextEditor(current));
    }
    onToolConsumed?.();
  }, [applyDraft, onCommit, onSelectedIdChange, onToolConsumed, openTextEditor, scale, shapes]);

  // ── Keyboard ────────────────────────────────────────────────────────────────
  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      if (editingText) return;
      if (event.key !== 'Delete' && event.key !== 'Backspace') return;
      const active = document.activeElement;
      if (active && ['INPUT', 'TEXTAREA'].includes(active.tagName)) return;
      if (!selectedId) return;
      event.preventDefault();
      onCommit(shapes.filter((shape) => shape.id !== selectedId));
      onSelectedIdChange(null);
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [editingText, onCommit, onSelectedIdChange, selectedId, shapes]);

  // ── Arrow endpoint handles ──────────────────────────────────────────────────
  const arrowHandles = useMemo(() => {
    if (!selectedShape || selectedShape.type !== 'arrow') return null;
    const [x1, y1, x2, y2] = selectedShape.points;
    return [
      { key: 'start', x: x1, y: y1, index: 0 },
      { key: 'end', x: x2, y: y2, index: 2 },
    ];
  }, [selectedShape]);

  const moveArrowHandle = useCallback(
    (index: number, node: Konva.Node) => {
      if (!selectedShape || selectedShape.type !== 'arrow') return;
      const next = [...selectedShape.points] as ArrowShape['points'];
      next[index] = node.x();
      next[index + 1] = node.y();
      replaceShape(selectedShape.id, (current) => ({ ...current, points: next }) as AnnotationShape);
    },
    [replaceShape, selectedShape],
  );

  const visibleShapes = draft ? [...shapes, draft] : shapes;
  const interactive = tool === 'select';

  return (
    <div ref={containerRef} className="relative flex w-full items-center justify-center">
      {!image && <div className="py-16 text-sm text-muted-foreground">Loading image…</div>}
      {image && (
        <div className="relative" style={{ width: stageWidth, height: stageHeight }}>
          <Stage
            ref={stageRef}
            width={stageWidth}
            height={stageHeight}
            onPointerDown={handlePointerDown}
            onPointerMove={handlePointerMove}
            onPointerUp={handlePointerUp}
            style={{ cursor: interactive ? 'default' : 'crosshair', touchAction: 'none' }}
          >
            <Layer ref={layerRef} scaleX={scale} scaleY={scale}>
              <KonvaImage image={image} x={0} y={0} width={naturalWidth} height={naturalHeight} listening={false} />
              {visibleShapes.map((shape) => (
                <AnnotationShapeNode
                  key={shape.id}
                  shape={shape}
                  scale={scale}
                  draggable={interactive}
                  onSelect={interactive ? onSelectedIdChange : undefined}
                  onDragEnd={interactive ? handleDragEnd : undefined}
                  onDoubleClick={interactive ? handleDoubleClick : undefined}
                />
              ))}
              {arrowHandles?.map((handle) => (
                <Circle
                  key={handle.key}
                  x={handle.x}
                  y={handle.y}
                  radius={7 / scale}
                  fill="#ffffff"
                  stroke="#2563eb"
                  strokeWidth={2 / scale}
                  draggable
                  onDragEnd={(event) => moveArrowHandle(handle.index, event.target)}
                />
              ))}
              <Transformer
                ref={transformerRef}
                rotateEnabled
                ignoreStroke
                anchorSize={8 / scale}
                borderStrokeWidth={1 / scale}
                onTransformEnd={handleTransformEnd}
                boundBoxFunc={(oldBox, newBox) => (newBox.width < 8 || newBox.height < 8 ? oldBox : newBox)}
              />
            </Layer>
          </Stage>

          {editingText && (
            <textarea
              autoFocus
              value={editingText.value}
              onChange={(event) => setEditingText({ ...editingText, value: event.target.value })}
              onBlur={commitTextEditor}
              onKeyDown={(event) => {
                if (event.key === 'Escape') {
                  event.preventDefault();
                  commitTextEditor();
                }
                if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
                  event.preventDefault();
                  commitTextEditor();
                }
              }}
              className="absolute z-10 resize-none rounded border border-primary bg-background/95 p-1 leading-tight shadow-sm outline-none"
              style={{
                left: editingText.left,
                top: editingText.top,
                width: editingText.width,
                fontSize: editingText.fontSize,
                fontWeight: 600,
              }}
            />
          )}
        </div>
      )}
    </div>
  );
});

/** Rejects click-sized drags so a stray click does not litter the image with zero-size shapes. */
function isDrawnShapeUsable(shape: AnnotationShape, scale: number): boolean {
  const minimum = MIN_DRAG_PIXELS / Math.max(scale, 0.01);
  switch (shape.type) {
    case 'arrow': {
      const [x1, y1, x2, y2] = shape.points;
      return Math.hypot(x2 - x1, y2 - y1) >= minimum;
    }
    case 'freehand':
      return shape.points.length >= 6;
    case 'ellipse':
      return shape.radiusX >= minimum / 2 && shape.radiusY >= minimum / 2;
    case 'text':
      return true;
    default:
      return shape.width >= minimum && shape.height >= minimum;
  }
}
