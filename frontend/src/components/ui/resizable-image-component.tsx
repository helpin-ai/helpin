import { useCallback, useEffect, useRef, useState } from 'react';
import { NodeViewWrapper } from '@tiptap/react';
import type { NodeViewProps } from '@tiptap/react';
import { Maximize2, Download, Copy, Link2, Trash2, X } from 'lucide-react';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { LoadingImage } from '@/components/ui/loading-image';
import { useImageActions } from '@/hooks/useImageActions';

const MIN_WIDTH = 100;

export function ResizableImageComponent({ node, updateAttributes, selected, deleteNode }: NodeViewProps) {
  const { src, alt, width, height, aspectRatio: storedAspectRatio } = node.attrs;
  const { copyImage, downloadImage, openInNewTab } = useImageActions();

  const containerRef = useRef<HTMLDivElement>(null);
  const imageRef = useRef<HTMLImageElement>(null);

  const [currentWidth, setCurrentWidth] = useState<string>(width ?? '35%');
  const [currentHeight, setCurrentHeight] = useState<string>(height ?? 'auto');
  const [aspectRatio, setAspectRatio] = useState<number | null>(storedAspectRatio ?? null);
  const [isResizing, setIsResizing] = useState(false);
  const [isFullscreen, setIsFullscreen] = useState(false);

  const containerRectRef = useRef<DOMRect | null>(null);
  const aspectRatioRef = useRef(aspectRatio);
  aspectRatioRef.current = aspectRatio;

  // On image load, compute aspect ratio and initial pixel size
  const handleImageLoad = useCallback(() => {
    const img = imageRef.current;
    if (!img) return;

    const ar = img.naturalWidth / img.naturalHeight;
    setAspectRatio(ar);
    aspectRatioRef.current = ar;

    // If width is still the default percentage, convert to pixels
    if (width === '100%' || width === '35%' || !width) {
      const editorContainer = img.closest('.overflow-hidden');
      const editorWidth = editorContainer?.clientWidth ?? 600;
      const naturalWidth = img.naturalWidth || editorWidth;
      const initialWidth = Math.max(Math.min(naturalWidth, editorWidth), MIN_WIDTH);
      const initialHeight = Math.round(initialWidth / ar);

      setCurrentWidth(`${initialWidth}px`);
      setCurrentHeight(`${initialHeight}px`);
      updateAttributes({
        width: `${initialWidth}px`,
        height: `${initialHeight}px`,
        aspectRatio: ar,
      });
    } else if (!storedAspectRatio) {
      updateAttributes({ aspectRatio: ar });
    }
  }, [width, storedAspectRatio, updateAttributes]);

  // Resize handler
  const handleResize = useCallback((e: MouseEvent | TouchEvent) => {
    if (!containerRectRef.current || !aspectRatioRef.current) return;

    const clientX = 'touches' in e ? e.touches[0].clientX : e.clientX;
    const newWidth = Math.max(clientX - containerRectRef.current.left, MIN_WIDTH);
    const newHeight = Math.round(newWidth / aspectRatioRef.current);

    setCurrentWidth(`${Math.round(newWidth)}px`);
    setCurrentHeight(`${newHeight}px`);
  }, []);

  // We need a ref-based version for the cleanup
  const currentSizeRef = useRef({ width: currentWidth, height: currentHeight });
  currentSizeRef.current = { width: currentWidth, height: currentHeight };

  const handleResizeEndRef = useRef(() => {
    setIsResizing(false);
    updateAttributes({
      width: currentSizeRef.current.width,
      height: currentSizeRef.current.height,
    });
  });
  handleResizeEndRef.current = () => {
    setIsResizing(false);
    updateAttributes({
      width: currentSizeRef.current.width,
      height: currentSizeRef.current.height,
    });
  };

  useEffect(() => {
    if (!isResizing) return;

    const onMove = (e: MouseEvent | TouchEvent) => handleResize(e);
    const onEnd = () => handleResizeEndRef.current();

    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onEnd);
    window.addEventListener('mouseleave', onEnd);
    window.addEventListener('touchmove', onMove);
    window.addEventListener('touchend', onEnd);

    return () => {
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onEnd);
      window.removeEventListener('mouseleave', onEnd);
      window.removeEventListener('touchmove', onMove);
      window.removeEventListener('touchend', onEnd);
    };
  }, [isResizing, handleResize]);

  const handleResizeStart = useCallback((e: React.MouseEvent | React.TouchEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (containerRef.current) {
      containerRectRef.current = containerRef.current.getBoundingClientRect();
    }
    setIsResizing(true);
  }, []);

  // Sync from external attribute changes (e.g. undo/redo)
  useEffect(() => {
    if (!isResizing) {
      setCurrentWidth(width ?? '35%');
      setCurrentHeight(height ?? 'auto');
    }
  }, [width, height, isResizing]);

  // Close fullscreen on Escape
  useEffect(() => {
    if (!isFullscreen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setIsFullscreen(false);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [isFullscreen]);

  return (
    <NodeViewWrapper className="relative my-2" data-drag-handle>
      <div
        ref={containerRef}
        className="group/img relative inline-block max-w-full"
        style={{
          width: currentWidth,
          ...(aspectRatio ? { aspectRatio: String(aspectRatio) } : {}),
        }}
      >
        <LoadingImage
          ref={imageRef}
          src={src}
          alt={alt ?? ''}
          crossOrigin="anonymous"
          onLoad={handleImageLoad}
          draggable={false}
          containerClassName="block max-w-full overflow-hidden rounded-md"
          className="block max-w-full rounded-md"
          style={{
            width: currentWidth,
            ...(aspectRatio ? { aspectRatio: String(aspectRatio) } : {}),
          }}
        />

        {/* Selection border — blue, visible on select or hover */}
        <div
          className={`pointer-events-none absolute inset-0 rounded-md transition-opacity duration-100 ${
            selected || isResizing ? 'opacity-100' : 'opacity-0 group-hover/img:opacity-100'
          }`}
          style={{ boxShadow: '0 0 0 2.5px #3b82f6', borderRadius: '0.375rem' }}
        />

        {/* Floating toolbar — inside image, top-right corner */}
        <div
          className={`absolute top-2 right-2 flex items-center rounded-lg border border-border bg-popover/95 shadow-md backdrop-blur-sm transition-opacity duration-100 ${
            isResizing
              ? 'pointer-events-none opacity-0'
              : selected
                ? 'pointer-events-auto opacity-100'
                : 'pointer-events-none opacity-0 group-hover/img:pointer-events-auto group-hover/img:opacity-100'
          }`}
        >
          <QuickTooltip label="View full size">
            <button
              type="button"
              onClick={(e) => { e.stopPropagation(); setIsFullscreen(true); }}
              className="flex h-8 w-8 items-center justify-center text-muted-foreground transition-colors hover:text-foreground"
            >
              <Maximize2 className="h-4 w-4" />
            </button>
          </QuickTooltip>
          <QuickTooltip label="Download">
            <button
              type="button"
              onClick={(e) => { e.stopPropagation(); downloadImage(src, alt); }}
              className="flex h-8 w-8 items-center justify-center text-muted-foreground transition-colors hover:text-foreground"
            >
              <Download className="h-4 w-4" />
            </button>
          </QuickTooltip>
          <QuickTooltip label="Copy image">
            <button
              type="button"
              onClick={(e) => { e.stopPropagation(); copyImage(src); }}
              className="flex h-8 w-8 items-center justify-center text-muted-foreground transition-colors hover:text-foreground"
            >
              <Copy className="h-4 w-4" />
            </button>
          </QuickTooltip>
          <QuickTooltip label="Open in new tab">
            <button
              type="button"
              onClick={(e) => { e.stopPropagation(); openInNewTab(src); }}
              className="flex h-8 w-8 items-center justify-center text-muted-foreground transition-colors hover:text-foreground"
            >
              <Link2 className="h-4 w-4" />
            </button>
          </QuickTooltip>
          <div className="mx-0.5 h-4 w-px bg-border" />
          <QuickTooltip label="Delete">
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                deleteNode();
              }}
              className="flex h-8 w-8 items-center justify-center text-muted-foreground transition-colors hover:text-destructive"
            >
              <Trash2 className="h-4 w-4" />
            </button>
          </QuickTooltip>
        </div>

        {/* Resize handle — bottom-right corner */}
        <div
          className={`absolute bottom-0 right-0 h-4 w-4 translate-x-1/2 translate-y-1/2 cursor-nwse-resize rounded-full border-2 border-white bg-primary shadow-sm transition-opacity duration-100 ${
            isResizing
              ? 'pointer-events-auto opacity-100'
              : 'pointer-events-none opacity-0 group-hover/img:pointer-events-auto group-hover/img:opacity-100'
          }`}
          onMouseDown={handleResizeStart}
          onTouchStart={handleResizeStart}
        />
      </div>

      {/* Fullscreen overlay */}
      {isFullscreen && (
        <div
          className="fixed inset-0 z-[9999] flex items-center justify-center bg-black/80 backdrop-blur-sm"
          onClick={() => setIsFullscreen(false)}
        >
          <QuickTooltip label="Close (Esc)">
            <button
              type="button"
              onClick={() => setIsFullscreen(false)}
              className="absolute top-4 right-4 flex h-10 w-10 items-center justify-center rounded-full bg-white/10 text-white transition-colors hover:bg-white/20"
            >
              <X className="h-5 w-5" />
            </button>
          </QuickTooltip>
          <LoadingImage
            src={src}
            alt={alt ?? ''}
            containerClassName="max-h-[90vh] max-w-[90vw] overflow-hidden rounded-lg"
            className="max-h-[90vh] max-w-[90vw] rounded-lg object-contain"
            onClick={(e) => e.stopPropagation()}
          />
        </div>
      )}
    </NodeViewWrapper>
  );
}
