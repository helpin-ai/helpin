import { useCallback, useLayoutEffect, useMemo, useRef, useState } from 'react';
import type { Editor } from '@tiptap/core';

// Own the editor lock and result together so a late response cannot replace
// a different conversation's draft or unlock a newer request.
export function useComposerRewrite(editor: Editor | null, scope: string) {
  const owner = useMemo(() => ({ editor, scope }), [editor, scope]);
  const [pendingOwner, setPendingOwner] = useState<typeof owner | null>(null);
  const isRewriting = pendingOwner === owner;
  const requestRef = useRef<object | null>(null);

  useLayoutEffect(() => {
    return () => {
      if (requestRef.current) {
        requestRef.current = null;
        if (owner.editor && !owner.editor.isDestroyed) owner.editor.setEditable(true, false);
      }
    };
  }, [owner]);

  const runRewrite = useCallback(async (request: () => Promise<string>): Promise<boolean> => {
    if (!editor || editor.isDestroyed || !editor.isEditable || requestRef.current) return false;
    const currentRequest = {};
    const originalDocument = editor.state.doc;
    requestRef.current = currentRequest;
    editor.setEditable(false, false);
    setPendingOwner(owner);
    try {
      const content = await request();
      if (requestRef.current !== currentRequest || editor.isDestroyed || !editor.state.doc.eq(originalDocument)) return false;
      editor.commands.setContent(content);
      return true;
    } finally {
      if (requestRef.current === currentRequest) {
        requestRef.current = null;
        if (!editor.isDestroyed) editor.setEditable(true, false);
        setPendingOwner(null);
      }
    }
  }, [editor, owner]);

  return { isRewriting, runRewrite };
}
