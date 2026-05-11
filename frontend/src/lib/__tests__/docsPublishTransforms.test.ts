// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest';
import { prepareDocsContentForPublish } from '@/lib/docsPublishTransforms';
import { renderMermaidSvg } from '@/lib/mermaidRenderer';

vi.mock('@/lib/mermaidRenderer', () => ({
  renderMermaidSvg: vi.fn(async (_source: string, theme: 'light' | 'dark') => `<svg><text>${theme}</text></svg>`),
  mermaidSvgFile: vi.fn((svg: string, filename: string) => new File([svg], filename, { type: 'image/svg+xml' })),
}));

vi.mock('@/lib/excalidrawRenderer', () => ({
  normalizeExcalidrawScene: vi.fn((scene) => ({
    elements: Array.isArray(scene?.elements) ? scene.elements : [],
    appState: scene?.appState ?? {},
    files: {},
  })),
  excalidrawPngFile: vi.fn(async (_scene, filename: string) => new File(['png'], filename, { type: 'image/png' })),
}));

vi.mock('@/hooks/useEditorImageUpload', () => ({
  uploadEditorImage: vi.fn(async (file: File) => ({
    attachmentId: file.name.includes('dark') ? 'att-dark' : 'att-light',
    publicUrl: `https://cdn.example.com/${file.name}`,
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
      { uploadConfig, diagramTheme: { brandColor: '#ef4444' } },
    );

    expect(result?.content?.[0].type).toBe('resizableImage');
    expect(String(result?.content?.[0].attrs?.src)).toContain('-light.svg');
    expect(String(result?.content?.[0].attrs?.darkSrc)).toContain('-dark.svg');
    expect(result?.content?.[0].attrs?.attachmentId).toBe('att-light');
    expect(result?.content?.[0].attrs?.darkAttachmentId).toBe('att-dark');
    expect(renderMermaidSvg).toHaveBeenCalledWith('graph TD\nA-->B', 'light', { brandColor: '#ef4444' });
    expect(renderMermaidSvg).toHaveBeenCalledWith('graph TD\nA-->B', 'dark', { brandColor: '#ef4444' });
    expect(result?.content?.[0].attrs?.publishedFrom).toMatchObject({
      type: 'codeBlock',
      attrs: { language: 'mermaid' },
    });
  });

  it('replaces Excalidraw blocks with PNG image snapshots', async () => {
    const source = {
      type: 'excalidraw',
      attrs: {
        title: 'Checkout flow',
        scene: {
          elements: [{ id: 'shape-1', type: 'rectangle' }],
          appState: { viewBackgroundColor: '#ffffff' },
          files: {},
        },
      },
    };

    const result = await prepareDocsContentForPublish(
      {
        type: 'doc',
        content: [source],
      },
      { uploadConfig },
    );

    expect(result?.content?.[0].type).toBe('resizableImage');
    expect(result?.content?.[0].attrs?.alt).toBe('Checkout flow');
    expect(String(result?.content?.[0].attrs?.src)).toContain('-light.png');
    expect(String(result?.content?.[0].attrs?.darkSrc)).toContain('-dark.png');
    expect(result?.content?.[0].attrs?.attachmentId).toBe('att-light');
    expect(result?.content?.[0].attrs?.darkAttachmentId).toBe('att-dark');
    expect(result?.content?.[0].attrs?.publishedFrom).toMatchObject({
      type: 'excalidraw',
      attrs: {
        title: 'Checkout flow',
        scene: { elements: [{ id: 'shape-1', type: 'rectangle' }], files: {} },
      },
    });
  });

  it('leaves empty Excalidraw blocks as renderable placeholders', async () => {
    const result = await prepareDocsContentForPublish(
      {
        type: 'doc',
        content: [
          {
            type: 'excalidraw',
            attrs: { title: 'Empty sketch', scene: { elements: [], appState: {}, files: {} } },
          },
        ],
      },
      { uploadConfig },
    );

    expect(result?.content?.[0]).toEqual({
      type: 'excalidraw',
      attrs: { title: 'Empty sketch', scene: { elements: [], appState: {}, files: {} } },
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
