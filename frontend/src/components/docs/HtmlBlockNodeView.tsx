import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { NodeViewWrapper, type NodeViewProps } from '@tiptap/react';
import { SourceCodeIcon, ViewIcon, PencilEdit01Icon, Delete01Icon, Alert01Icon } from '@/lib/icons';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { sanitizeHtml } from './htmlSanitizer';

export function HtmlBlockNodeView({ node, updateAttributes, deleteNode, editor, getPos }: NodeViewProps) {
  const { html, renderMode } = node.attrs;
  const editable = editor.isEditable;
  const hasContent = !!(html && html.trim());
  const sandboxed = renderMode === 'sandboxed';
  const [focused, setFocused] = useState(false);
  const [editing, setEditing] = useState(!hasContent && editable);
  const [draft, setDraft] = useState(html ?? '');
  const nodeViewRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Sanitize for preview
  const sanitized = useMemo(() => hasContent && !sandboxed ? sanitizeHtml(html) : '', [html, hasContent, sandboxed]);
  const wasStripped = hasContent && !sandboxed && sanitized.trim().length < html.trim().length;

  // Sync draft when node attrs change externally (undo/redo)
  useEffect(() => { setDraft(html ?? ''); }, [html]);

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
        if (editing) {
          updateAttributes({ html: draft });
          setEditing(false);
        }
        setFocused(false);
      }
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [focused, editing, draft, updateAttributes]);

  // Auto-focus and auto-resize textarea when switching to edit mode
  useEffect(() => {
    if (editing && textareaRef.current) {
      setTimeout(() => {
        const ta = textareaRef.current;
        if (ta) {
          ta.focus();
          ta.style.height = 'auto';
          ta.style.height = `${ta.scrollHeight}px`;
        }
      }, 50);
    }
  }, [editing]);

  const switchToEdit = useCallback(() => {
    setDraft(html ?? '');
    setEditing(true);
  }, [html]);

  const switchToPreview = useCallback(() => {
    updateAttributes({ html: draft });
    setEditing(false);
  }, [draft, updateAttributes]);

  return (
    <NodeViewWrapper>
      <div
        ref={nodeViewRef}
        onClick={() => { if (editable && !focused) setFocused(true); }}
        className={`docs-html-block group/html relative my-6 rounded-lg transition-all ${
          focused
            ? 'border border-border ring-2 ring-primary'
            : hasContent
              ? 'border border-transparent hover:border-border'
              : 'border border-dashed border-muted-foreground/30'
        }`}
      >
        {/* Header bar — when focused or empty */}
        {(focused || !hasContent) && (
          <div className="flex items-center justify-between rounded-t-lg bg-muted/50 px-3 py-1.5 text-xs">
            <span className="flex items-center gap-1.5 text-muted-foreground font-medium">
              <SourceCodeIcon className="h-3.5 w-3.5" />
              HTML Block
            </span>
            {editable && (
              <div className="flex items-center gap-0.5">
                {(hasContent || editing) && (
                  <QuickTooltip label={editing ? 'Preview' : 'Edit HTML'}>
                    <button
                      type="button"
                      onClick={(e) => { e.stopPropagation(); editing ? switchToPreview() : switchToEdit(); }}
                      className="flex h-6 w-6 items-center justify-center rounded text-muted-foreground hover:text-foreground cursor-pointer"
                    >
                      {editing ? <ViewIcon className="h-3 w-3" /> : <PencilEdit01Icon className="h-3 w-3" />}
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
        <div className={focused || !hasContent ? 'px-3 py-2' : ''}>
          {editing ? (
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
                    setEditing(false);
                  }
                }}
                onBlur={() => updateAttributes({ html: draft })}
                onKeyDown={(e) => {
                  e.stopPropagation();
                  // Escape also exits
                  if (e.key === 'Escape') {
                    updateAttributes({ html: draft });
                    setEditing(false);
                  }
                }}
                className="flex-1 min-h-0 py-2 px-3 font-mono text-sm outline-none resize-none leading-[1.625rem] bg-transparent"
                style={{ height: 'auto', overflow: 'hidden' }}
                placeholder="<div>&#10;  <p>Your HTML here...</p>&#10;</div>"
              />
            </div>
          ) : hasContent ? (
            <div
              className={sandboxed ? 'docs-html-block-raw max-w-none text-sm' : 'prose prose-sm dark:prose-invert max-w-none text-sm'}
              dangerouslySetInnerHTML={{ __html: sandboxed ? html : sanitized }}
              onDoubleClick={editable ? switchToEdit : undefined}
            />
          ) : (
            <div
              className="flex items-center justify-center gap-2 py-6 text-muted-foreground/60 cursor-pointer hover:text-muted-foreground transition-colors"
              onClick={editable ? switchToEdit : undefined}
            >
              <SourceCodeIcon className="h-4 w-4" />
              <span className="text-sm">Click to add HTML</span>
            </div>
          )}
        </div>

        {/* Sanitization warning */}
        {wasStripped && !editing && (
          <div className="flex items-center gap-1.5 border-t px-3 py-1.5 text-xs text-amber-600 dark:text-amber-400">
            <Alert01Icon className="h-3 w-3 shrink-0" />
            Some HTML was removed for safety. The preview shows the sanitized version.
          </div>
        )}
      </div>
    </NodeViewWrapper>
  );
}
