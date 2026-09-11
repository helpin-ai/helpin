// @vitest-environment jsdom
import { Editor } from '@tiptap/core';
import StarterKit from '@tiptap/starter-kit';
import { Markdown } from 'tiptap-markdown';
import { describe, expect, it } from 'vitest';

import { CodeBlockExtension } from '@/components/editor/CodeBlockExtension';

describe('shared TipTap editor diagram code blocks', () => {
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

  it.each([['mermaid', 'graph TD\n  A-->B'], ['nwdiag', 'nwdiag { network dmz { web01; } }']])('round-trips fenced %s markdown', (language, source) => {
    const editor = createEditor();
    try {
      editor.commands.setContent('```' + language + '\n' + source + '\n```');

      expect(editor.getJSON().content?.[0]).toMatchObject({
        type: 'codeBlock',
        attrs: { language },
      });
      expect(editor.getText()).toContain(source);
      expect(editor.storage.markdown.getMarkdown()).toContain('```' + language + '\n' + source);
      const html = editor.getHTML();
      editor.commands.setContent(html);
      expect(editor.getJSON().content?.[0]?.attrs?.language).toBe(language);
    } finally {
      editor.destroy();
    }
  });
});
