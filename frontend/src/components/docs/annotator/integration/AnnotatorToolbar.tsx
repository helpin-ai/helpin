import {
  ArrowMoveUpRightIcon,
  ArrowTurnBackwardIcon,
  ArrowTurnForwardIcon,
  BubbleChatIcon,
  CircleIcon,
  CursorPointer01Icon,
  Delete01Icon,
  HighlighterIcon,
  PenTool02Icon,
  SquareIcon,
  TextIcon,
  ViewOffIcon,
} from '@/lib/icons';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { cn } from '@/lib/utils';
import type { AnnotationTool } from '../core/ImageAnnotator';

const TOOLS: { tool: AnnotationTool; label: string; icon: typeof SquareIcon }[] = [
  { tool: 'select', label: 'Select', icon: CursorPointer01Icon },
  { tool: 'arrow', label: 'Arrow', icon: ArrowMoveUpRightIcon },
  { tool: 'rect', label: 'Box', icon: SquareIcon },
  { tool: 'ellipse', label: 'Ellipse', icon: CircleIcon },
  { tool: 'text', label: 'Text', icon: TextIcon },
  { tool: 'callout', label: 'Callout', icon: BubbleChatIcon },
  { tool: 'highlight', label: 'Highlight', icon: HighlighterIcon },
  { tool: 'cover', label: 'Redact', icon: ViewOffIcon },
  { tool: 'freehand', label: 'Draw', icon: PenTool02Icon },
];

const ANNOTATION_COLORS = ['#ef4444', '#f59e0b', '#22c55e', '#3b82f6', '#a855f7', '#111827', '#ffffff'];
const STROKE_WIDTHS = [2, 4, 8];
export const FONT_SIZE_PRESETS = [16, 24, 32, 48] as const;

interface AnnotatorToolbarProps {
  tool: AnnotationTool;
  onToolChange: (tool: AnnotationTool) => void;
  color: string;
  onColorChange: (color: string) => void;
  strokeWidth: number;
  onStrokeWidthChange: (width: number) => void;
  fontSize: number;
  onFontSizeChange: (size: number) => void;
  showFontSizes: boolean;
  canUndo: boolean;
  canRedo: boolean;
  onUndo: () => void;
  onRedo: () => void;
  canDelete: boolean;
  onDelete: () => void;
}

export function AnnotatorToolbar({
  tool,
  onToolChange,
  color,
  onColorChange,
  strokeWidth,
  onStrokeWidthChange,
  fontSize,
  onFontSizeChange,
  showFontSizes,
  canUndo,
  canRedo,
  onUndo,
  onRedo,
  canDelete,
  onDelete,
}: AnnotatorToolbarProps) {
  return (
    <div className="flex flex-wrap items-center gap-1 rounded-lg border bg-background p-1">
      {TOOLS.map(({ tool: value, label, icon: Icon }) => (
        <QuickTooltip key={value} label={label}>
          <button
            type="button"
            aria-label={label}
            aria-pressed={tool === value}
            onClick={() => onToolChange(value)}
            className={cn(
              'flex h-8 w-8 items-center justify-center rounded-md transition-colors',
              tool === value ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted hover:text-foreground',
            )}
          >
            <Icon className="h-4 w-4" />
          </button>
        </QuickTooltip>
      ))}

      <div className="mx-1 h-5 w-px bg-border" />

      {ANNOTATION_COLORS.map((value) => (
        <button
          key={value}
          type="button"
          aria-label={`Color ${value}`}
          aria-pressed={color === value}
          onClick={() => onColorChange(value)}
          className={cn(
            'h-5 w-5 rounded-full border transition-transform',
            color === value ? 'scale-110 ring-2 ring-primary ring-offset-1 ring-offset-background' : 'hover:scale-110',
          )}
          style={{ backgroundColor: value }}
        />
      ))}

      <div className="mx-1 h-5 w-px bg-border" />

      {STROKE_WIDTHS.map((value) => (
        <button
          key={value}
          type="button"
          aria-label={`Stroke ${value}`}
          aria-pressed={strokeWidth === value}
          onClick={() => onStrokeWidthChange(value)}
          className={cn(
            'flex h-8 w-8 items-center justify-center rounded-md transition-colors',
            strokeWidth === value ? 'bg-muted text-foreground' : 'text-muted-foreground hover:bg-muted',
          )}
        >
          <span className="rounded-full bg-current" style={{ width: value + 2, height: value + 2 }} />
        </button>
      ))}

      {showFontSizes && (
        <>
          <div className="mx-1 h-5 w-px bg-border" />
          {FONT_SIZE_PRESETS.map((value) => (
            <button
              key={value}
              type="button"
              aria-label={`Text size ${value}`}
              aria-pressed={fontSize === value}
              onClick={() => onFontSizeChange(value)}
              className={cn(
                'flex h-8 min-w-8 items-center justify-center rounded-md px-1.5 text-xs font-semibold transition-colors',
                fontSize === value ? 'bg-muted text-foreground' : 'text-muted-foreground hover:bg-muted hover:text-foreground',
              )}
            >
              {value}
            </button>
          ))}
        </>
      )}

      <div className="mx-1 h-5 w-px bg-border" />

      <QuickTooltip label="Undo">
        <button
          type="button"
          aria-label="Undo"
          disabled={!canUndo}
          onClick={onUndo}
          className="flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground disabled:pointer-events-none disabled:opacity-40"
        >
          <ArrowTurnBackwardIcon className="h-4 w-4" />
        </button>
      </QuickTooltip>
      <QuickTooltip label="Redo">
        <button
          type="button"
          aria-label="Redo"
          disabled={!canRedo}
          onClick={onRedo}
          className="flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground disabled:pointer-events-none disabled:opacity-40"
        >
          <ArrowTurnForwardIcon className="h-4 w-4" />
        </button>
      </QuickTooltip>
      <QuickTooltip label="Delete selected">
        <button
          type="button"
          aria-label="Delete selected"
          disabled={!canDelete}
          onClick={onDelete}
          className="flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-destructive disabled:pointer-events-none disabled:opacity-40"
        >
          <Delete01Icon className="h-4 w-4" />
        </button>
      </QuickTooltip>
    </div>
  );
}
