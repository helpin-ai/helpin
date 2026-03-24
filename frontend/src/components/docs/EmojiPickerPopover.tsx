import { useEffect, useRef, useState } from 'react';
import data from '@emoji-mart/data';
import Picker from '@emoji-mart/react';
import type { Editor } from '@tiptap/core';

interface EmojiPickerPopoverProps {
  editor: Editor;
  open: boolean;
  onClose: () => void;
  insertPos: number;
}

export function EmojiPickerPopover({ editor, open, onClose, insertPos }: EmojiPickerPopoverProps) {
  const containerRef = useRef<HTMLDivElement>(null);

  // Close on click outside
  useEffect(() => {
    if (!open) return;
    const handleClick = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        onClose();
      }
    };
    // Delay to avoid closing immediately from the slash menu click
    const timer = setTimeout(() => {
      document.addEventListener('mousedown', handleClick);
    }, 100);
    return () => {
      clearTimeout(timer);
      document.removeEventListener('mousedown', handleClick);
    };
  }, [open, onClose]);

  // Close on Escape
  useEffect(() => {
    if (!open) return;
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', handleKey);
    return () => document.removeEventListener('keydown', handleKey);
  }, [open, onClose]);

  // Position near cursor
  const [pos, setPos] = useState<{ top: number; left: number }>({ top: 0, left: 0 });
  useEffect(() => {
    if (!open) return;
    try {
      const coords = editor.view.coordsAtPos(insertPos);
      const wrapper = editor.view.dom.closest('.docs-editor-wrapper');
      if (wrapper) {
        const wr = wrapper.getBoundingClientRect();
        setPos({
          top: coords.bottom - wr.top + wrapper.scrollTop + 4,
          left: Math.max(0, coords.left - wr.left - 150),
        });
      }
    } catch { /* ignore */ }
  }, [open, insertPos, editor]);

  if (!open) return null;

  return (
    <div ref={containerRef} className="absolute z-50" style={{ top: pos.top, left: pos.left }}>
      <Picker
        data={data}
        onEmojiSelect={(emoji: any) => {
          editor.chain()
            .focus()
            .setTextSelection(insertPos)
            .insertContent(emoji.native)
            .run();
          onClose();
        }}
        theme="light"
        previewPosition="none"
        skinTonePosition="search"
        maxFrequentRows={2}
        perLine={8}
      />
    </div>
  );
}
