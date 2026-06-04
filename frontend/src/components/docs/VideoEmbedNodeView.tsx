import { useEffect, useRef, useState } from 'react';
import { NodeViewWrapper, type NodeViewProps } from '@tiptap/react';
import { LinkSquare01Icon, Delete01Icon } from '@/lib/icons';
import { QuickTooltip } from '@/components/ui/quick-tooltip';

const PROVIDER_LABELS: Record<string, string> = {
  youtube: 'YouTube',
  vimeo: 'Vimeo',
  loom: 'Loom',
  wistia: 'Wistia',
};

export function VideoEmbedNodeView({ node, deleteNode, editor, getPos }: NodeViewProps) {
  const { provider, sourceUrl, embedUrl } = node.attrs;
  const editable = editor.isEditable;
  const [focused, setFocused] = useState(false);
  const nodeViewRef = useRef<HTMLDivElement>(null);

  // Track selection
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

  // Click outside — unfocus
  useEffect(() => {
    if (!focused) return;
    const handleClick = (e: MouseEvent) => {
      if (nodeViewRef.current && !nodeViewRef.current.contains(e.target as Node)) {
        setFocused(false);
      }
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [focused]);

  return (
    <NodeViewWrapper>
      <div
        ref={nodeViewRef}
        onClick={() => { if (editable) setFocused(true); }}
        className={`docs-video-embed group/video relative my-6 rounded-lg overflow-hidden ${focused ? 'ring-2 ring-primary' : ''}`}
      >
        {/* Responsive iframe container — 16:9 */}
        <div className="relative w-full" style={{ paddingBottom: '56.25%' }}>
          <iframe
            src={embedUrl}
            className="absolute inset-0 w-full h-full"
            allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
            allowFullScreen
            sandbox="allow-scripts allow-same-origin allow-popups allow-presentation"
            referrerPolicy="strict-origin-when-cross-origin"
            title={`${PROVIDER_LABELS[provider] ?? 'Video'} embed`}
          />
        </div>

        {/* Toolbar — top-right on hover */}
        {editable && (
          <div className="absolute top-2 right-2 flex items-center rounded-lg border bg-popover/95 shadow-md backdrop-blur-sm opacity-0 group-hover/video:opacity-100 transition-opacity">
            <span className="px-2 py-1.5 text-xs text-muted-foreground">{PROVIDER_LABELS[provider] ?? 'Video'}</span>
            <QuickTooltip label="Open original">
              <button
                type="button"
                onClick={() => window.open(sourceUrl, '_blank', 'noopener,noreferrer')}
                className="flex h-8 w-8 items-center justify-center text-muted-foreground hover:text-foreground cursor-pointer"
              >
                <LinkSquare01Icon className="h-3.5 w-3.5" />
              </button>
            </QuickTooltip>
            <div className="mx-0.5 h-4 w-px bg-border" />
            <QuickTooltip label="Delete">
              <button
                type="button"
                onClick={() => deleteNode()}
                className="flex h-8 w-8 items-center justify-center text-muted-foreground hover:text-destructive cursor-pointer"
              >
                <Delete01Icon className="h-3.5 w-3.5" />
              </button>
            </QuickTooltip>
          </div>
        )}
      </div>
    </NodeViewWrapper>
  );
}
