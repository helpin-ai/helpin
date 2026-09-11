// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { describe, expect, it } from 'vitest';
import { Editor } from '@tiptap/core';
import StarterKit from '@tiptap/starter-kit';
import { Markdown } from 'tiptap-markdown';
import { SvgBlock } from '../SvgBlock';
import { CodeBlockExtension } from '../CodeBlockExtension';
import { HtmlBlockExtension } from '@/components/docs/HtmlBlockExtension';
import { slashCommands } from '@/components/docs/slash-commands';
import { SVG_EXAMPLE } from '@/lib/svgRenderer';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('SVG diagram editing', () => {
  it('shows errors and recovers to an isolated image with an accessible title', () => {
    const container = document.createElement('div');
    const root = createRoot(container);
    try {
      act(() => root.render(<SvgBlock source="<svg><script>alert(1)</script></svg>" />));
      expect(container.querySelector('[role="alert"]')?.textContent).toContain('static SVG');
      act(() => root.render(<SvgBlock source={SVG_EXAMPLE} />));
      expect(container.querySelector('[role="alert"]')).toBeNull();
      expect(container.querySelector('svg')).toBeNull();
      expect(container.querySelector('img')?.alt).toBe('Request flow');
      expect(container.querySelector('img')?.src).toContain('data:image/svg+xml;base64,');
    } finally { act(() => root.unmount()); }
  });

  it('inserts /svg as a code block and round-trips its source through Markdown', () => {
    const editor = new Editor({ extensions: [StarterKit.configure({ codeBlock: false }), CodeBlockExtension, Markdown], content: '' });
    try {
      slashCommands.find((command) => command.title === 'SVG')!.action(editor);
      const svg = editor.getJSON().content?.find((node) => node.type === 'codeBlock');
      expect(svg?.attrs?.language).toBe('svg');
      expect(svg?.content?.[0].text).toBe(SVG_EXAMPLE);
      const markdown = editor.storage.markdown.getMarkdown();
      expect(markdown).toContain('```svg\n' + SVG_EXAMPLE);
      editor.commands.setContent(markdown);
      expect(editor.getJSON().content?.find((node) => node.type === 'codeBlock')?.content?.[0].text).toBe(SVG_EXAMPLE);
    } finally { editor.destroy(); }
  });

  it('retains automatically isolated HTML source when exporting Markdown', () => {
    const html = '<style>.site{color:red}</style><div class="site">Site 1</div><script>window.diagram=true</script>';
    const editor = new Editor({ extensions: [StarterKit, HtmlBlockExtension, Markdown], content: { type: 'doc', content: [{ type: 'htmlBlock', attrs: { html } }] } });
    try {
      expect(editor.storage.markdown.getMarkdown()).toContain(html);
      expect(editor.storage.markdown.getMarkdown()).toContain('data-render-mode="sandboxed"');
    } finally { editor.destroy(); }
  });
});
