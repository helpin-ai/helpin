import { useState, useEffect, useRef } from 'react';
import { NodeViewContent, NodeViewWrapper, type NodeViewProps } from '@tiptap/react';
import type { CalloutVariant } from './CalloutExtension';

const VARIANT_STYLES: Record<CalloutVariant, { bg: string; border: string }> = {
  blue: { bg: 'bg-blue-50 dark:bg-blue-950/30', border: 'border-l-blue-400' },
  green: { bg: 'bg-green-50 dark:bg-green-950/30', border: 'border-l-green-500' },
  grey: { bg: 'bg-stone-100 dark:bg-stone-900/30', border: 'border-l-stone-400' },
  red: { bg: 'bg-red-50 dark:bg-red-950/30', border: 'border-l-red-500' },
  yellow: { bg: 'bg-amber-50 dark:bg-amber-950/30', border: 'border-l-amber-400' },
};

const VARIANT_DOTS: { key: CalloutVariant; color: string }[] = [
  { key: 'yellow', color: '#d97706' },
  { key: 'blue', color: '#3b82f6' },
  { key: 'green', color: '#16a34a' },
  { key: 'red', color: '#7f1d1d' },
  { key: 'grey', color: '#9ca3af' },
];

export function CalloutNodeView({ node, updateAttributes, editor, getPos }: NodeViewProps) {
  const variant = (node.attrs.variant as CalloutVariant) || 'grey';
  const styles = VARIANT_STYLES[variant];
  const editable = editor.isEditable;
  const [focused, setFocused] = useState(false);
  const nodeViewRef = useRef<HTMLElement>(null);

  // Track whether cursor is inside THIS specific callout
  useEffect(() => {
    const update = () => {
      const pos = typeof getPos === 'function' ? getPos() : undefined;
      if (pos === undefined || pos === null) { setFocused(false); return; }
      try {
      const { from } = editor.state.selection;
      const nodeEnd = pos + node.nodeSize;
      const isFocused = from > pos && from < nodeEnd;
      setFocused(prev => prev === isFocused ? prev : isFocused);
      } catch { setFocused(false); }
    };
    editor.on('selectionUpdate', update);
    return () => { editor.off('selectionUpdate', update); };
  }, [editor, node.nodeSize, getPos]);

  // Click outside — unfocus (handles clicks in margins outside editor DOM)
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
      <aside
        ref={nodeViewRef}
        className={`relative my-3 rounded-md border-l-4 px-4 py-3 transition-shadow ${styles.bg} ${styles.border} ${
          focused ? 'ring-2 ring-primary/40' : ''
        }`}
      >
        {editable && focused && (
          <div className="absolute -top-3 right-2 flex items-center gap-1.5 rounded-full bg-popover border shadow-sm px-2 py-1">
            {VARIANT_DOTS.map(({ key, color }) => (
              <button
                key={key}
                type="button"
                className={`h-4 w-4 rounded-full transition-all ${
                  key === variant ? 'scale-125 opacity-100' : 'opacity-40 hover:opacity-80'
                }`}
                style={{ backgroundColor: color }}
                onClick={() => updateAttributes({ variant: key })}
                title={key.charAt(0).toUpperCase() + key.slice(1)}
              />
            ))}
          </div>
        )}
        <div className="min-w-0 [&>*:last-child]:mb-0">
          <NodeViewContent />
        </div>
      </aside>
    </NodeViewWrapper>
  );
}
