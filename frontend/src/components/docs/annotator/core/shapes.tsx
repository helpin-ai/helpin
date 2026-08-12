import { Arrow, Ellipse, Group, Line, Rect, Text } from 'react-konva';
import type Konva from 'konva';
import {
  HIGHLIGHT_OPACITY,
  type AnnotationShape,
  type CalloutShape,
} from './annotationTypes';

export interface ShapeRenderProps {
  shape: AnnotationShape;
  /** Interactive stage only — the offscreen export render passes none of these. */
  draggable?: boolean;
  onSelect?: (id: string) => void;
  /**
   * Receives the konva node itself, not a position: box shapes carry their coordinates on the
   * node, while arrows, freehand and callouts are drawn at absolute points and so report a
   * drag *delta*. The caller resolves which it is and resets the node.
   */
  onDragEnd?: (id: string, node: Konva.Node) => void;
  onDoubleClick?: (id: string) => void;
  /** Display scale, used to keep hit strokes usable when the image is scaled down. */
  scale?: number;
}

const CALLOUT_PADDING = 12;
const CALLOUT_CORNER = 8;

function calloutTailPoints(shape: CalloutShape): number[] {
  // Anchor the tail on the box edge nearest the tip so the tail never crosses the body.
  const centerY = shape.y + shape.height / 2;
  const anchorX = Math.max(shape.x, Math.min(shape.tailX, shape.x + shape.width));
  const anchorY = shape.tailY < centerY ? shape.y : shape.y + shape.height;
  const spread = Math.min(18, shape.width / 3);
  return [
    anchorX - spread,
    anchorY,
    shape.tailX,
    shape.tailY,
    anchorX + spread,
    anchorY,
  ];
}

/**
 * Renders one annotation shape. Coordinates are in source-image pixel space — the parent Layer
 * carries the display scale, so nothing here needs to know how large the stage is.
 */
export function AnnotationShapeNode({
  shape,
  draggable = false,
  onSelect,
  onDragEnd,
  onDoubleClick,
  scale = 1,
}: ShapeRenderProps) {
  const hitStroke = Math.max(12, 12 / scale);

  const common = {
    id: shape.id,
    name: 'annotation-shape',
    draggable,
    onMouseDown: onSelect ? () => onSelect(shape.id) : undefined,
    onTap: onSelect ? () => onSelect(shape.id) : undefined,
    onDragEnd: onDragEnd
      ? (event: Konva.KonvaEventObject<DragEvent>) => onDragEnd(shape.id, event.target)
      : undefined,
    onDblClick: onDoubleClick ? () => onDoubleClick(shape.id) : undefined,
    onDblTap: onDoubleClick ? () => onDoubleClick(shape.id) : undefined,
  };

  switch (shape.type) {
    case 'arrow':
      return (
        <Arrow
          {...common}
          points={shape.points}
          stroke={shape.color}
          fill={shape.color}
          strokeWidth={shape.strokeWidth}
          pointerLength={shape.strokeWidth * 3}
          pointerWidth={shape.strokeWidth * 3}
          lineCap="round"
          lineJoin="round"
          hitStrokeWidth={hitStroke}
        />
      );

    case 'rect':
      return (
        <Rect
          {...common}
          x={shape.x}
          y={shape.y}
          width={shape.width}
          height={shape.height}
          rotation={shape.rotation}
          stroke={shape.color}
          strokeWidth={shape.strokeWidth}
          cornerRadius={2}
          hitStrokeWidth={hitStroke}
        />
      );

    case 'ellipse':
      return (
        <Ellipse
          {...common}
          x={shape.x}
          y={shape.y}
          radiusX={shape.radiusX}
          radiusY={shape.radiusY}
          rotation={shape.rotation}
          stroke={shape.color}
          strokeWidth={shape.strokeWidth}
          hitStrokeWidth={hitStroke}
        />
      );

    case 'text':
      return (
        <Text
          {...common}
          x={shape.x}
          y={shape.y}
          text={shape.text}
          fill={shape.color}
          fontSize={shape.fontSize}
          fontStyle="bold"
          rotation={shape.rotation}
          fontFamily="Inter, system-ui, sans-serif"
        />
      );

    case 'callout':
      return (
        <Group {...common}>
          <Line points={calloutTailPoints(shape)} closed fill={shape.color} stroke={shape.color} strokeWidth={1} />
          <Rect
            x={shape.x}
            y={shape.y}
            width={shape.width}
            height={shape.height}
            fill={shape.color}
            cornerRadius={CALLOUT_CORNER}
          />
          <Text
            x={shape.x + CALLOUT_PADDING}
            y={shape.y + CALLOUT_PADDING}
            width={shape.width - CALLOUT_PADDING * 2}
            height={shape.height - CALLOUT_PADDING * 2}
            text={shape.text}
            fill="#ffffff"
            fontSize={16}
            fontStyle="bold"
            fontFamily="Inter, system-ui, sans-serif"
            wrap="word"
          />
        </Group>
      );

    case 'highlight':
      return (
        <Rect
          {...common}
          x={shape.x}
          y={shape.y}
          width={shape.width}
          height={shape.height}
          fill={shape.color}
          opacity={HIGHLIGHT_OPACITY}
        />
      );

    case 'cover':
      // Opaque by design — this is a redaction, not a visual effect.
      return (
        <Rect
          {...common}
          x={shape.x}
          y={shape.y}
          width={shape.width}
          height={shape.height}
          fill={shape.color}
          opacity={1}
        />
      );

    case 'freehand':
      return (
        <Line
          {...common}
          points={shape.points}
          stroke={shape.color}
          strokeWidth={shape.strokeWidth}
          lineCap="round"
          lineJoin="round"
          tension={0.3}
          hitStrokeWidth={hitStroke}
        />
      );

    default:
      return null;
  }
}
