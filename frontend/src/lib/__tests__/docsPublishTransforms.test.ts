// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest';
import { prepareDocsContentForPublish } from '@/lib/docsPublishTransforms';

vi.mock('@/lib/mermaidRenderer', () => ({
  renderMermaidSvg: vi.fn(async () => '<svg><text>ok</text></svg>'),
  mermaidSvgFile: vi.fn((svg: string) => new File([svg], 'diagram.svg', { type: 'image/svg+xml' })),
}));

vi.mock('@/hooks/useEditorImageUpload', () => ({
  uploadEditorImage: vi.fn(async () => ({
    attachmentId: 'att-1',
    publicUrl: 'https://cdn.example.com/diagram.svg',
  })),
}));

describe('prepareDocsContentForPublish', () => {
  const uploadConfig = {
    workspaceId: 'ws-1',
    entityType: 'editor_upload' as const,
    entityId: 'doc-1',
  };

  it('replaces Mermaid code blocks with image snapshots', async () => {
    const result = await prepareDocsContentForPublish(
      {
        type: 'doc',
        content: [
          {
            type: 'codeBlock',
            attrs: { language: 'mermaid' },
            content: [{ type: 'text', text: 'graph TD\nA-->B' }],
          },
        ],
      },
      { uploadConfig },
    );

    expect(result?.content?.[0].type).toBe('resizableImage');
    expect(result?.content?.[0].attrs?.src).toBe('https://cdn.example.com/diagram.svg');
    expect(result?.content?.[0].attrs?.publishedFrom).toMatchObject({
      type: 'codeBlock',
      attrs: { language: 'mermaid' },
    });
  });

  it('leaves regular code blocks and html blocks unchanged', async () => {
    const content = {
      type: 'doc',
      content: [
        { type: 'codeBlock', attrs: { language: 'go' }, content: [{ type: 'text', text: 'fmt.Println()' }] },
        { type: 'htmlBlock', attrs: { html: '<div>Safe</div>' } },
      ],
    };

    const result = await prepareDocsContentForPublish(content, { uploadConfig });

    expect(result).toEqual(content);
  });
});
