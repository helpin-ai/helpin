import { useEffect, useRef, useState, useCallback } from 'react';
import type { Editor } from '@tiptap/core';
import { PlusSignIcon } from '@/lib/icons';

interface BlockGapInserterProps {
  editor: Editor;
}

// Block node types where gaps should be insertable
const BLOCK_TYPES = new Set(['resizableImage', 'callout', 'table', 'codeBlock', 'blockquote', 'horizontalRule']);

interface GapPosition {
  top: number;
  left: number;
  width: number;
  insertPos: number;
}

export function BlockGapInserter({ editor }: BlockGapInserterProps) {
  const [gap, setGap] = useState<GapPosition | null>(null);
  const wrapperRef = useRef<HTMLElement | null>(null);
  const hideTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const btnRef = useRef<HTMLButtonElement>(null);

  const getWrapper = useCallback(() => {
    if (!wrapperRef.current) {
      wrapperRef.current = editor.view.dom.closest('.docs-editor-wrapper') as HTMLElement;
    }
    return wrapperRef.current;
  }, [editor]);

  useEffect(() => {
    const wrapper = getWrapper();
    if (!wrapper) return;

    let rafPending = false;
    const editorDom = editor.view.dom;

    const handleMouseMove = (e: MouseEvent) => {
      const target = e.target as HTMLElement;

      // If hovering over the inserter button itself, keep it visible
      if (btnRef.current?.contains(target)) {
        if (hideTimerRef.current) {
          clearTimeout(hideTimerRef.current);
          hideTimerRef.current = null;
        }
        return;
      }

      if (!editorDom.contains(target)) {
        scheduleHide();
        return;
      }

      // Throttle with RAF to avoid expensive doc traversal on every mousemove
      if (rafPending) return;
      rafPending = true;
      requestAnimationFrame(() => {
        rafPending = false;
        findGap(e.clientY);
      });
    };

    const findGap = (mouseY: number) => {
      const { doc } = editor.state;
      const wr = wrapper.getBoundingClientRect();
      const scrollTop = wrapper.scrollTop;

      let bestGap: GapPosition | null = null;
      let bestDist = 20;

      doc.forEach((node, offset, index) => {
        if (index === 0) return;

        const prevNode = doc.child(index - 1);
        const currIsBlock = BLOCK_TYPES.has(node.type.name);
        const prevIsBlock = BLOCK_TYPES.has(prevNode.type.name);

        if (!currIsBlock && !prevIsBlock) return;

        try {
          const prevDomNode = editor.view.nodeDOM(offset - prevNode.nodeSize);
          const currDomNode = editor.view.nodeDOM(offset);

          if (!(prevDomNode instanceof HTMLElement) || !(currDomNode instanceof HTMLElement)) return;

          const prevBottom = prevDomNode.getBoundingClientRect().bottom;
          const currTop = currDomNode.getBoundingClientRect().top;
          const gapCenter = (prevBottom + currTop) / 2;
          const dist = Math.abs(mouseY - gapCenter);

          if (dist < bestDist) {
            bestDist = dist;
            const editorRect = editorDom.getBoundingClientRect();
            bestGap = {
              top: gapCenter - wr.top + scrollTop,
              left: editorRect.left - wr.left,
              width: editorRect.width,
              insertPos: offset,
            };
          }
        } catch {
          // nodeDOM can throw for some node types
        }
      });

      if (bestGap) {
        if (hideTimerRef.current) {
          clearTimeout(hideTimerRef.current);
          hideTimerRef.current = null;
        }
        setGap(bestGap);
      } else {
        scheduleHide();
      }
    };

    const scheduleHide = () => {
      if (!hideTimerRef.current) {
        hideTimerRef.current = setTimeout(() => {
          setGap(null);
          hideTimerRef.current = null;
        }, 400);
      }
    };

    wrapper.addEventListener('mousemove', handleMouseMove);
    return () => {
      wrapper.removeEventListener('mousemove', handleMouseMove);
      if (hideTimerRef.current) clearTimeout(hideTimerRef.current);
    };
  }, [editor, getWrapper]);

  const insertParagraph = useCallback(() => {
    if (!gap) return;
    editor
      .chain()
      .focus()
      .insertContentAt(gap.insertPos, { type: 'paragraph' })
      .setTextSelection(gap.insertPos + 1)
      .run();
    setGap(null);
  }, [editor, gap]);

  if (!gap) return null;

  return (
    <button
      ref={btnRef}
      className="absolute z-20 flex items-center justify-center cursor-pointer group"
      style={{
        left: gap.left,
        top: gap.top - 8,
        width: gap.width,
        height: 16,
      }}
      onMouseDown={(e) => e.preventDefault()}
      onClick={insertParagraph}
      title="Add paragraph"
    >
      {/* Line */}
      <div className="absolute inset-x-4 top-1/2 h-px bg-primary/0 group-hover:bg-primary/30 transition-colors" />
      {/* Plus icon */}
      <div className="flex h-5 w-5 items-center justify-center rounded-full bg-transparent text-transparent group-hover:bg-primary/10 group-hover:text-primary/60 transition-colors">
        <PlusSignIcon className="h-3 w-3" />
      </div>
    </button>
  );
}
