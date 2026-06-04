import { useCallback, useEffect, useRef, useState } from 'react';
import { NodeViewWrapper } from '@tiptap/react';
import type { NodeViewProps } from '@tiptap/react';
import { TextAlignLeftIcon, TextAlignCenterIcon, TextAlignRightIcon, Maximize01Icon, Download04Icon, Copy01Icon, Link01Icon, Delete01Icon, Cancel01Icon, Tick01Icon, CursorTextIcon } from '@/lib/icons';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { LoadingImage } from '@/components/ui/loading-image';
import { useImageActions } from '@/hooks/useImageActions';

const MIN_WIDTH = 100;

const ALIGNMENT_CLASS: Record<string, string> = {
  left: 'items-start',
  center: 'items-center',
  right: 'items-end',
};

const ALIGNMENT_OPTIONS = [
  { value: 'left', label: 'Left', icon: TextAlignLeftIcon },
  { value: 'center', label: 'Center', icon: TextAlignCenterIcon },
  { value: 'right', label: 'Right', icon: TextAlignRightIcon },
] as const;

export function ResizableImageComponent({ node, updateAttributes, selected: _selected, deleteNode, editor, extension }: NodeViewProps) {
  const { src, darkSrc, alt, caption, width, height, aspectRatio: storedAspectRatio, alignment, linkUrl, linkNewTab } = node.attrs;
  const { copyImage, downloadImage, openInNewTab: _openInNewTab } = useImageActions();
  const enableCaption = extension.options.enableCaption ?? true;

  const containerRef = useRef<HTMLDivElement>(null);
  const imageRef = useRef<HTMLImageElement>(null);

  const [currentWidth, setCurrentWidth] = useState<string>(width ?? '35%');
  const [currentHeight, setCurrentHeight] = useState<string>(height ?? 'auto');
  const [aspectRatio, setAspectRatio] = useState<number | null>(storedAspectRatio ?? null);
  const [isResizing, setIsResizing] = useState(false);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [showAlignMenu, setShowAlignMenu] = useState(false);
  const [showAltInput, setShowAltInput] = useState(false);
  const [altText, setAltText] = useState<string>(alt ?? '');
  const [captionText, setCaptionText] = useState<string>(caption ?? '');
  const [linkInput, setLinkInput] = useState<string>(linkUrl ?? '');
  const [linkNewTabInput, setLinkNewTabInput] = useState<boolean>(linkNewTab ?? true);

  const settingsRef = useRef<HTMLDivElement>(null);

  const containerRectRef = useRef<DOMRect | null>(null);
  const aspectRatioRef = useRef(aspectRatio);
  aspectRatioRef.current = aspectRatio;

  const editable = editor?.isEditable ?? true;

  // Sync from external changes
  useEffect(() => {
    setAltText(alt ?? '');
    setCaptionText(caption ?? '');
    setLinkInput(linkUrl ?? '');
    setLinkNewTabInput(linkNewTab ?? true);
  }, [alt, caption, linkUrl, linkNewTab]);

  // On image load, compute aspect ratio and initial pixel size
  const handleImageLoad = useCallback(() => {
    const img = imageRef.current;
    if (!img) return;

    const ar = img.naturalWidth / img.naturalHeight;
    setAspectRatio(ar);
    aspectRatioRef.current = ar;

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

  // Close settings on click outside
  useEffect(() => {
    if (!showSettings) return;
    const handleClick = (e: MouseEvent) => {
      if (settingsRef.current && !settingsRef.current.contains(e.target as Node)) {
        setShowSettings(false);
      }
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [showSettings]);

  const saveAlt = () => {
    updateAttributes({ alt: altText || null });
  };

  const saveLink = () => {
    const url = linkInput.trim();
    updateAttributes({
      linkUrl: url || null,
      linkNewTab: linkNewTabInput,
    });
  };

  const clearLink = () => {
    setLinkInput('');
    updateAttributes({ linkUrl: null, linkNewTab: true });
  };

  const currentAlignment = alignment || 'center';

  return (
    <NodeViewWrapper className={`docs-image-block relative my-6 flex flex-col ${ALIGNMENT_CLASS[currentAlignment] ?? 'items-center'}`} data-drag-handle>
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
          onLoad={handleImageLoad}
          draggable={false}
          containerClassName={`${darkSrc ? 'dark:hidden' : 'block'} max-w-full overflow-hidden rounded-md`}
          className="block max-w-full rounded-md"
          style={{
            width: currentWidth,
            ...(aspectRatio ? { aspectRatio: String(aspectRatio) } : {}),
          }}
        />
        {darkSrc && (
          <LoadingImage
            src={darkSrc}
            alt={alt ?? ''}
            draggable={false}
            containerClassName="hidden max-w-full overflow-hidden rounded-md dark:block"
            className="block max-w-full rounded-md"
            style={{
              width: currentWidth,
              ...(aspectRatio ? { aspectRatio: String(aspectRatio) } : {}),
            }}
          />
        )}

        {/* Selection border — only on hover or resize, not on programmatic selection */}
        <div
          className={`pointer-events-none absolute inset-0 rounded-md transition-opacity duration-100 ${
            isResizing ? 'opacity-100' : 'opacity-0 group-hover/img:opacity-100'
          }`}
          style={{ boxShadow: '0 0 0 2.5px #3b82f6', borderRadius: '0.375rem' }}
        />

        {/* Floating toolbar — top-right */}
        {editable && (
          <div
            className={`absolute top-2 right-2 flex items-center rounded-lg border border-border bg-popover/95 shadow-md backdrop-blur-sm transition-opacity duration-100 ${
              isResizing
                ? 'pointer-events-none opacity-0'
                : 'pointer-events-none opacity-0 group-hover/img:pointer-events-auto group-hover/img:opacity-100'
            }`}
          >
            {/* Alignment dropdown */}
            <div className="relative">
              <QuickTooltip label="Alignment">
                <button
                  type="button"
                  onClick={(e) => { e.stopPropagation(); setShowAlignMenu(!showAlignMenu); setShowAltInput(false); setShowSettings(false); }}
                  className="flex h-8 w-8 items-center justify-center text-muted-foreground transition-colors hover:text-foreground"
                >
                  {currentAlignment === 'left' ? <TextAlignLeftIcon className="h-4 w-4" /> :
                   currentAlignment === 'right' ? <TextAlignRightIcon className="h-4 w-4" /> :
                   <TextAlignCenterIcon className="h-4 w-4" />}
                </button>
              </QuickTooltip>
              {showAlignMenu && (
                <div className="absolute top-9 left-0 z-50 w-32 rounded-lg border bg-popover p-1 shadow-md">
                  {ALIGNMENT_OPTIONS.map(({ value, label, icon: Icon }) => (
                    <button
                      key={value}
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation();
                        updateAttributes({ alignment: value });
                        setShowAlignMenu(false);
                      }}
                      className={`flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm transition-colors ${
                        currentAlignment === value ? 'text-primary bg-accent' : 'text-muted-foreground hover:bg-accent/50 hover:text-foreground'
                      }`}
                    >
                      <Icon className="h-3.5 w-3.5" />
                      {label}
                    </button>
                  ))}
                </div>
              )}
            </div>

            {/* Alt text */}
            <div className="relative">
              <QuickTooltip label="Alt text">
                <button
                  type="button"
                  onClick={(e) => { e.stopPropagation(); setShowAltInput(!showAltInput); setShowAlignMenu(false); setShowSettings(false); }}
                  className={`flex h-8 w-8 items-center justify-center transition-colors ${showAltInput ? 'text-primary' : altText ? 'text-foreground' : 'text-muted-foreground hover:text-foreground'}`}
                >
                  <CursorTextIcon className="h-4 w-4" />
                </button>
              </QuickTooltip>
              {showAltInput && (
                <div className="absolute top-9 left-1/2 -translate-x-1/2 z-50 w-56 rounded-lg border bg-popover p-2 shadow-md" onClick={(e) => e.stopPropagation()} onMouseDown={(e) => e.stopPropagation()}>
                  <input
                    type="text"
                    value={altText}
                    onChange={(e) => setAltText(e.target.value)}
                    onBlur={saveAlt}
                    onKeyDown={(e) => { if (e.key === 'Enter') { saveAlt(); setShowAltInput(false); } }}
                    placeholder="Describe this image..."
                    className="w-full rounded-md border bg-background px-2.5 py-1.5 text-sm outline-none focus:ring-2 focus:ring-primary/30"
                    autoFocus
                  />
                </div>
              )}
            </div>

            <div className="mx-0.5 h-4 w-px bg-border" />

            <QuickTooltip label="View full size">
              <button
                type="button"
                onClick={(e) => { e.stopPropagation(); setIsFullscreen(true); }}
                className="flex h-8 w-8 items-center justify-center text-muted-foreground transition-colors hover:text-foreground"
              >
                <Maximize01Icon className="h-4 w-4" />
              </button>
            </QuickTooltip>
            <QuickTooltip label="Download">
              <button
                type="button"
                onClick={(e) => { e.stopPropagation(); downloadImage(src, alt); }}
                className="flex h-8 w-8 items-center justify-center text-muted-foreground transition-colors hover:text-foreground"
              >
                <Download04Icon className="h-4 w-4" />
              </button>
            </QuickTooltip>
            <QuickTooltip label="Copy image">
              <button
                type="button"
                onClick={(e) => { e.stopPropagation(); copyImage(src); }}
                className="flex h-8 w-8 items-center justify-center text-muted-foreground transition-colors hover:text-foreground"
              >
                <Copy01Icon className="h-4 w-4" />
              </button>
            </QuickTooltip>
            <QuickTooltip label="Add link">
              <button
                type="button"
                onClick={(e) => { e.stopPropagation(); setShowSettings(!showSettings); setShowAlignMenu(false); setShowAltInput(false); }}
                className={`flex h-8 w-8 items-center justify-center transition-colors ${showSettings ? 'text-primary' : 'text-muted-foreground hover:text-foreground'}`}
              >
                <Link01Icon className="h-4 w-4" />
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
                <Delete01Icon className="h-4 w-4" />
              </button>
            </QuickTooltip>
          </div>
        )}

        {/* Settings panel — below toolbar */}
        {editable && showSettings && (
          <div
            ref={settingsRef}
            className="absolute top-12 right-2 z-50 w-72 rounded-lg border bg-popover p-3 shadow-lg space-y-3"
            onClick={(e) => e.stopPropagation()}
            onMouseDown={(e) => e.stopPropagation()}
          >
            {/* Link URL */}
            <div className="space-y-1">
              <label className="text-xs font-medium text-muted-foreground">Link URL</label>
              <div className="flex gap-1.5">
                <input
                  type="url"
                  value={linkInput}
                  onChange={(e) => setLinkInput(e.target.value)}
                  onBlur={saveLink}
                  onKeyDown={(e) => { if (e.key === 'Enter') { saveLink(); } }}
                  placeholder="https://..."
                  className="flex-1 rounded-md border bg-background px-2.5 py-1.5 text-sm outline-none focus:ring-2 focus:ring-primary/30"
                />
                {linkInput && (
                  <button
                    type="button"
                    onClick={clearLink}
                    className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md border text-muted-foreground hover:text-destructive"
                  >
                    <Cancel01Icon className="h-3.5 w-3.5" />
                  </button>
                )}
              </div>
            </div>

            {/* Open in new tab */}
            <button
              type="button"
              className="flex items-center gap-2 cursor-pointer"
              onClick={() => {
                const newVal = !linkNewTabInput;
                setLinkNewTabInput(newVal);
                updateAttributes({ linkNewTab: newVal });
              }}
            >
              <span
                className={`flex h-4 w-4 items-center justify-center rounded-sm border transition-colors ${
                  linkNewTabInput ? 'bg-primary border-primary text-primary-foreground' : 'border-muted-foreground/40'
                }`}
              >
                {linkNewTabInput && <Tick01Icon className="h-3 w-3" />}
              </span>
              <span className="text-sm">Open in new tab</span>
            </button>
          </div>
        )}

        {/* Resize handle */}
        {editable && (
          <div
            className={`absolute bottom-0 right-0 h-4 w-4 translate-x-1/2 translate-y-1/2 cursor-nwse-resize rounded-full border-2 border-white bg-primary shadow-sm transition-opacity duration-100 ${
              isResizing
                ? 'pointer-events-auto opacity-100'
                : 'pointer-events-none opacity-0 group-hover/img:pointer-events-auto group-hover/img:opacity-100'
            }`}
            onMouseDown={handleResizeStart}
            onTouchStart={handleResizeStart}
          />
        )}
      </div>
      {enableCaption && (editable || captionText) && (
        <input
          value={captionText}
          onChange={(event) => {
            setCaptionText(event.target.value);
            updateAttributes({ caption: event.target.value.trim() || null });
          }}
          placeholder="Add caption"
          readOnly={!editable}
          className="mt-2 w-full max-w-[min(100%,32rem)] bg-transparent text-center text-xs text-muted-foreground outline-none placeholder:text-muted-foreground/60"
          contentEditable={false}
        />
      )}

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
              <Cancel01Icon className="h-5 w-5" />
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
