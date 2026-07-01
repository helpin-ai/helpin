// @vitest-environment jsdom
import { Editor } from '@tiptap/core';
import StarterKit from '@tiptap/starter-kit';
import { Markdown } from 'tiptap-markdown';
import { describe, expect, it } from 'vitest';

import { CodeBlockExtension } from '@/components/editor/CodeBlockExtension';

describe('shared TipTap editor Mermaid code blocks', () => {
  function createEditor() {
    return new Editor({
      extensions: [
        StarterKit.configure({
          heading: { levels: [1, 2, 3] },
          codeBlock: false,
        }),
        CodeBlockExtension,
        Markdown.configure({
          html: true,
          tightLists: true,
          bulletListMarker: '-',
          transformPastedText: true,
          transformCopiedText: false,
        }),
      ],
      content: '<p></p>',
    });
  }

  it('parses fenced Mermaid markdown into a Mermaid code block', () => {
    const editor = createEditor();
    try {
      editor.commands.setContent('```mermaid\ngraph TD\n  A-->B\n```');

      expect(editor.getJSON().content?.[0]).toMatchObject({
        type: 'codeBlock',
        attrs: { language: 'mermaid' },
      });
      expect(editor.getText()).toContain('graph TD');
    } finally {
      editor.destroy();
    }
  });
});
