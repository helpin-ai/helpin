import { useState, useEffect, useRef } from 'react';
import { NodeViewContent, NodeViewWrapper, type NodeViewProps } from '@tiptap/react';
import { normalizeCalloutVariant, type SemanticCalloutVariant } from './CalloutExtension';

const VARIANT_STYLES: Record<SemanticCalloutVariant, { bg: string; border: string }> = {
  info: { bg: 'bg-blue-50 dark:bg-blue-950/30', border: 'border-l-blue-400' },
  warning: { bg: 'bg-amber-50 dark:bg-amber-950/30', border: 'border-l-amber-400' },
  tip: { bg: 'bg-emerald-50 dark:bg-emerald-950/30', border: 'border-l-emerald-500' },
  danger: { bg: 'bg-red-50 dark:bg-red-950/30', border: 'border-l-red-500' },
  success: { bg: 'bg-green-50 dark:bg-green-950/30', border: 'border-l-green-500' },
};

const VARIANT_DOTS: { key: SemanticCalloutVariant; color: string; label: string }[] = [
  { key: 'info', color: '#3b82f6', label: 'Info' },
  { key: 'warning', color: '#d97706', label: 'Warning' },
  { key: 'tip', color: '#059669', label: 'Tip' },
  { key: 'danger', color: '#dc2626', label: 'Danger' },
  { key: 'success', color: '#16a34a', label: 'Success' },
];

export function CalloutNodeView({ node, updateAttributes, editor, getPos }: NodeViewProps) {
  const variant = normalizeCalloutVariant(node.attrs.variant as string | undefined);
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
        className={`docs-callout relative my-5 rounded-md border-l-4 px-4 py-3 transition-shadow ${styles.bg} ${styles.border} ${
          focused ? 'ring-2 ring-primary/40' : ''
        }`}
      >
        {editable && focused && (
          <div className="absolute -top-3 right-2 flex items-center gap-1.5 rounded-full bg-popover border shadow-sm px-2 py-1">
            {VARIANT_DOTS.map(({ key, color, label }) => (
              <button
                key={key}
                type="button"
                className={`h-4 w-4 rounded-full transition-all ${
                  key === variant ? 'scale-125 opacity-100' : 'opacity-40 hover:opacity-80'
                }`}
                style={{ backgroundColor: color }}
                onClick={() => updateAttributes({ variant: key })}
                title={label}
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
