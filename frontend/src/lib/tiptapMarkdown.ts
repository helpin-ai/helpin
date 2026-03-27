import { Editor } from '@tiptap/core';
import StarterKit from '@tiptap/starter-kit';
import { Markdown } from 'tiptap-markdown';

function createMarkdownEditor(content: string) {
  return new Editor({
    extensions: [
      StarterKit.configure({
        heading: { levels: [1, 2, 3] },
        link: {
          openOnClick: false,
          HTMLAttributes: { class: 'text-primary underline cursor-pointer' },
        },
      }),
      Markdown.configure({
        html: true,
        tightLists: true,
        bulletListMarker: '-',
        transformPastedText: true,
        transformCopiedText: false,
      }),
    ],
    content: content || '<p></p>',
  });
}

export function htmlToMarkdown(html: string) {
  const editor = createMarkdownEditor(html.trim());
  try {
    return ((editor.storage as unknown as { markdown: { getMarkdown: () => string } }).markdown)
      .getMarkdown()
      .trim();
  } finally {
    editor.destroy();
  }
}

export function markdownToHtml(markdown: string) {
  const editor = createMarkdownEditor('<p></p>');
  try {
    const trimmed = markdown.trim();
    if (!trimmed) return '';
    editor.commands.setContent(trimmed);
    const html = editor.getHTML().replace(/(?:<p><\/p>)+$/, '');
    return html === '<p></p>' ? '' : html;
  } finally {
    editor.destroy();
  }
}
