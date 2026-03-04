import { useCallback, useEffect, useRef, useState } from 'react';
import { NodeViewWrapper } from '@tiptap/react';
import type { NodeViewProps } from '@tiptap/react';

const MIN_WIDTH = 100;

export function ResizableImageComponent({ node, updateAttributes, selected }: NodeViewProps) {
  const { src, alt, width, height, aspectRatio: storedAspectRatio } = node.attrs;

  const containerRef = useRef<HTMLDivElement>(null);
  const imageRef = useRef<HTMLImageElement>(null);

  const [currentWidth, setCurrentWidth] = useState<string>(width ?? '35%');
  const [currentHeight, setCurrentHeight] = useState<string>(height ?? 'auto');
  const [aspectRatio, setAspectRatio] = useState<number | null>(storedAspectRatio ?? null);
  const [isResizing, setIsResizing] = useState(false);

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
    if (width === '35%' || !width) {
      const editorContainer = img.closest('.overflow-hidden');
      const editorWidth = editorContainer?.clientWidth ?? 600;
      const initialWidth = Math.max(Math.round(editorWidth * 0.35), MIN_WIDTH);
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

  const handleResizeEnd = useCallback(() => {
    setIsResizing(false);
    // Persist to node attributes
    updateAttributes({
      width: currentWidth,
      height: currentHeight,
    });
  }, [currentWidth, currentHeight, updateAttributes]);

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
        <img
          ref={imageRef}
          src={src}
          alt={alt ?? ''}
          onLoad={handleImageLoad}
          draggable={false}
          className={`block max-w-full rounded-md transition-[filter] duration-100 ${
            selected ? 'ring-2 ring-primary brightness-95' : ''
          }`}
          style={{
            width: currentWidth,
            ...(aspectRatio ? { aspectRatio: String(aspectRatio) } : {}),
          }}
        />

        {/* Selection border */}
        <div
          className={`pointer-events-none absolute inset-0 rounded-md border-2 border-primary transition-opacity duration-100 ${
            isResizing ? 'opacity-100' : 'opacity-0 group-hover/img:opacity-100'
          }`}
        />

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
    </NodeViewWrapper>
  );
}
