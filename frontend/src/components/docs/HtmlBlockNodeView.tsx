import { useCallback, useEffect, useId, useMemo, useRef, useState } from 'react';
import { NodeViewWrapper, type NodeViewProps } from '@tiptap/react';
import { Copy01Icon, Delete01Icon, SourceCodeIcon, Tick01Icon, ViewIcon } from '@/lib/icons';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { sanitizeHtml } from './htmlSanitizer';
import { buildHtmlBlockSrcDoc, shouldRenderHtmlBlockIsolated } from './htmlBlockRendering';

export function HtmlBlockNodeView({ node, updateAttributes, deleteNode, editor, getPos }: NodeViewProps) {
  const { html, renderMode } = node.attrs;
  const editable = editor.isEditable;
  const hasContent = !!(html && html.trim());
  const isolated = hasContent && shouldRenderHtmlBlockIsolated(html, renderMode);
  const frameId = useId();
  const [focused, setFocused] = useState(false);
  const [mode, setMode] = useState<'rendered' | 'source'>(hasContent ? 'rendered' : 'source');
  const [draft, setDraft] = useState(html ?? '');
  const [copied, setCopied] = useState(false);
  const [frameHeight, setFrameHeight] = useState(720);
  const nodeViewRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Sanitize for preview
  const sanitized = useMemo(() => hasContent && !isolated ? sanitizeHtml(html) : '', [html, hasContent, isolated]);
  const srcDoc = useMemo(() => isolated ? buildHtmlBlockSrcDoc(html, frameId) : '', [frameId, html, isolated]);

  // Sync draft when node attrs change externally (undo/redo)
  useEffect(() => { setDraft(html ?? ''); }, [html]);

  useEffect(() => {
    if (!isolated) return;
    const handleMessage = (event: MessageEvent) => {
      const data = event.data as { type?: string; id?: string; height?: number } | null;
      if (!data || data.type !== 'helpin:html-block:resize' || data.id !== frameId) return;
      if (typeof data.height !== 'number' || !Number.isFinite(data.height)) return;
      setFrameHeight(Math.max(180, Math.min(Math.ceil(data.height), 2400)));
    };
    window.addEventListener('message', handleMessage);
    return () => window.removeEventListener('message', handleMessage);
  }, [frameId, isolated]);

  // Track selection inside this node
  useEffect(() => {
    const update = () => {
      const pos = typeof getPos === 'function' ? getPos() : undefined;
      if (pos === undefined || pos === null) { setFocused(false); return; }
      try {
        const { from } = editor.state.selection;
        const nodeEnd = pos + node.nodeSize;
        setFocused(prev => {
          const f = from >= pos && from <= nodeEnd;
          return prev === f ? prev : f;
        });
      } catch { setFocused(false); }
    };
    editor.on('selectionUpdate', update);
    return () => { editor.off('selectionUpdate', update); };
  }, [editor, node.nodeSize, getPos]);

  // Click outside — unfocus and save
  useEffect(() => {
    if (!focused) return;
    const handleClick = (e: MouseEvent) => {
      if (nodeViewRef.current && !nodeViewRef.current.contains(e.target as Node)) {
        if (mode === 'source') {
          updateAttributes({ html: draft });
        }
        setFocused(false);
      }
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [focused, mode, draft, updateAttributes]);

  // Auto-focus and auto-resize textarea when switching to edit mode
  useEffect(() => {
    if (mode === 'source' && editable && textareaRef.current) {
      setTimeout(() => {
        const ta = textareaRef.current;
        if (ta) {
          ta.focus();
          ta.style.height = 'auto';
          ta.style.height = `${ta.scrollHeight}px`;
        }
      }, 50);
    }
  }, [editable, mode]);

  const switchToSource = useCallback(() => {
    setDraft(html ?? '');
    setMode('source');
  }, [html]);

  const switchToRendered = useCallback(() => {
    updateAttributes({ html: draft });
    setMode('rendered');
  }, [draft, updateAttributes]);

  const handleCopy = useCallback(() => {
    const value = mode === 'source' ? draft : html;
    const write = navigator.clipboard?.writeText?.(value || '');
    if (!write) return;
    write.then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }, [draft, html, mode]);

  const showChrome = editable;
  const showSource = editable && mode === 'source';

  if (!editable && !hasContent) {
    return <NodeViewWrapper />;
  }

  return (
    <NodeViewWrapper>
      <div
        ref={nodeViewRef}
        onClick={() => { if (editable && !focused) setFocused(true); }}
        className={`docs-html-block group/html relative my-6 rounded-lg transition-all ${
          focused
            ? 'border border-border ring-2 ring-primary'
            : hasContent
              ? editable ? 'border border-border/60 bg-muted/20' : 'border border-transparent'
              : 'border border-dashed border-muted-foreground/30'
        }`}
      >
        {showChrome && (
          <div className="flex items-center justify-between gap-2 rounded-t-lg border-b border-border/50 bg-muted/50 px-3 py-1.5 text-xs">
            <span className="flex items-center gap-1.5 text-muted-foreground font-medium">
              <SourceCodeIcon className="h-3.5 w-3.5" />
              HTML Block
            </span>
            {editable && (
              <div className="flex items-center gap-1">
                <div className="mr-1 flex items-center gap-1 rounded border border-border/60 bg-background/70 p-0.5" contentEditable={false}>
                  <button
                    type="button"
                    onClick={(e) => { e.stopPropagation(); switchToRendered(); }}
                    className={`flex h-6 items-center gap-1 rounded px-2 text-xs transition-colors ${
                      mode === 'rendered'
                        ? 'bg-accent text-foreground'
                        : 'text-muted-foreground hover:text-foreground'
                    }`}
                    title="Show rendered HTML"
                  >
                    <ViewIcon className="h-3 w-3" />
                    Rendered
                  </button>
                  <button
                    type="button"
                    onClick={(e) => { e.stopPropagation(); switchToSource(); }}
                    className={`flex h-6 items-center gap-1 rounded px-2 text-xs transition-colors ${
                      mode === 'source'
                        ? 'bg-accent text-foreground'
                        : 'text-muted-foreground hover:text-foreground'
                    }`}
                    title="Show HTML source"
                  >
                    <SourceCodeIcon className="h-3 w-3" />
                    Source
                  </button>
                </div>
                {hasContent && (
                  <QuickTooltip label={copied ? 'Copied' : 'Copy HTML'}>
                    <button
                      type="button"
                      onClick={(e) => { e.stopPropagation(); handleCopy(); }}
                      className="flex h-6 w-6 items-center justify-center rounded text-muted-foreground hover:text-foreground cursor-pointer"
                    >
                      {copied ? <Tick01Icon className="h-3 w-3" /> : <Copy01Icon className="h-3 w-3" />}
                    </button>
                  </QuickTooltip>
                )}
                <QuickTooltip label="Delete">
                  <button
                    type="button"
                    onClick={(e) => { e.stopPropagation(); deleteNode(); }}
                    className="flex h-6 w-6 items-center justify-center rounded text-muted-foreground hover:text-destructive cursor-pointer"
                  >
                    <Delete01Icon className="h-3 w-3" />
                  </button>
                </QuickTooltip>
              </div>
            )}
          </div>
        )}

        {/* Inline editor or preview */}
        <div className={showChrome ? 'px-3 py-2' : ''}>
          {showSource ? (
            <div className="flex rounded-md border bg-background focus-within:ring-2 focus-within:ring-primary/30 overflow-hidden">
              {/* Line numbers */}
              <div
                className="select-none border-r bg-muted/50 px-2 py-2 text-right font-mono text-xs text-muted-foreground/50 leading-[1.625rem]"
                aria-hidden
              >
                {(draft || '\n').split('\n').map((_line: string, i: number) => (
                  <div key={i}>{i + 1}</div>
                ))}
              </div>
              <textarea
                ref={textareaRef}
                value={draft}
                onChange={(e) => {
                  const val = e.target.value;
                  setDraft(val);
                  // Auto-resize
                  const ta = e.target;
                  ta.style.height = 'auto';
                  ta.style.height = `${ta.scrollHeight}px`;
                  // Exit on 3rd Enter — same pattern as callout
                  if (val.endsWith('\n\n\n')) {
                    const trimmed = val.replace(/\n+$/, '');
                    updateAttributes({ html: trimmed });
                    setDraft(trimmed);
                    setMode('rendered');
                  }
                }}
                onBlur={() => updateAttributes({ html: draft })}
                onKeyDown={(e) => {
                  e.stopPropagation();
                  // Escape also exits
                  if (e.key === 'Escape') {
                    updateAttributes({ html: draft });
                    setMode('rendered');
                  }
                }}
                className="flex-1 min-h-0 py-2 px-3 font-mono text-sm outline-none resize-none leading-[1.625rem] bg-transparent"
                style={{ height: 'auto', overflow: 'hidden' }}
                placeholder="<div>&#10;  <p>Your HTML here...</p>&#10;</div>"
              />
            </div>
          ) : hasContent ? (
            isolated ? (
              <iframe
                title="Rendered HTML block"
                className="docs-html-block-frame block w-full rounded-md border border-border bg-white"
                sandbox="allow-scripts allow-popups allow-forms allow-presentation"
                referrerPolicy="no-referrer"
                srcDoc={srcDoc}
                style={{ height: `${frameHeight}px` }}
                onDoubleClick={editable ? switchToSource : undefined}
              />
            ) : (
              <div
                className="prose prose-sm dark:prose-invert max-w-none text-sm"
                dangerouslySetInnerHTML={{ __html: sanitized }}
                onDoubleClick={editable ? switchToSource : undefined}
              />
            )
          ) : (
            <div
              className="flex items-center justify-center gap-2 py-6 text-muted-foreground/60 cursor-pointer hover:text-muted-foreground transition-colors"
              onClick={editable ? switchToSource : undefined}
            >
              <SourceCodeIcon className="h-4 w-4" />
              <span className="text-sm">Click to add HTML</span>
            </div>
          )}
        </div>
      </div>
    </NodeViewWrapper>
  );
}
