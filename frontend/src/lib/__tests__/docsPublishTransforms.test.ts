// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest';
import { prepareDocsContentForPublish } from '@/lib/docsPublishTransforms';
import { renderMermaidSvg } from '@/lib/mermaidRenderer';
import { renderAnnotationsToFile } from '@/components/docs/annotator/core/renderAnnotations';
import { automationService } from '@/lib/services/automationService';

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

vi.mock('@/components/docs/annotator/core/renderAnnotations', () => ({
  renderAnnotationsToFile: vi.fn(async (_sourceUrl: string, _state: unknown, filename: string) =>
    new File(['annotated'], filename, { type: 'image/png' })),
}));

vi.mock('@/lib/services/automationService', () => ({
  automationService: {
    getArtifactContentURL: vi.fn(async () => ({
      data: { url: 'https://artifacts.example.com/original.png', expires_at: '2099-01-01T00:00:00Z' },
      error: null,
    })),
  },
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

  it('publishes an annotated legacy artifact as a fresh flattened image', async () => {
    const annotationState = {
      version: 1,
      baseWidth: 1440,
      baseHeight: 900,
      shapes: [{ id: 'shape-1', type: 'text', x: 20, y: 30, width: 260, text: 'Note', color: '#ef4444', fontSize: 24, rotation: 0 }],
    };
    const result = await prepareDocsContentForPublish({
      type: 'doc',
      content: [{
        type: 'resizableImage',
        attrs: {
          src: 'https://cdn.example.com/draft-preview.png',
          artifactId: 'artifact-1',
          darkSrc: 'https://cdn.example.com/original-dark.png',
          annotationState,
          sourceAttachmentId: 'stale-derived-attachment',
          width: '60%',
        },
      }],
    }, { uploadConfig });

    const published = result?.content?.[0];
    expect(automationService.getArtifactContentURL).toHaveBeenCalledWith('ws-1', 'artifact-1');
    expect(renderAnnotationsToFile).toHaveBeenCalledWith(
      'https://artifacts.example.com/original.png',
      annotationState,
      'image-published-annotated.png',
    );
    expect(published?.attrs).toMatchObject({
      src: 'https://cdn.example.com/image-published-annotated.png',
      attachmentId: 'att-light',
      width: '60%',
    });
    expect(published?.attrs).not.toHaveProperty('annotationState');
    expect(published?.attrs).not.toHaveProperty('sourceAttachmentId');
    expect(published?.attrs).not.toHaveProperty('artifactId');
    expect(published?.attrs).not.toHaveProperty('darkSrc');
  });

  it('resolves the original artifact when no attachment source exists', async () => {
    const annotationState = {
      version: 1,
      baseWidth: 800,
      baseHeight: 600,
      shapes: [{ id: 'shape-1', type: 'rect', x: 10, y: 10, width: 100, height: 80, color: '#ef4444', strokeWidth: 4, rotation: 0 }],
    };
    await prepareDocsContentForPublish({
      type: 'doc',
      content: [{ type: 'resizableImage', attrs: { src: '/rendered.png', artifactId: 'artifact-1', annotationState } }],
    }, { uploadConfig });

    expect(automationService.getArtifactContentURL).toHaveBeenCalledWith('ws-1', 'artifact-1');
    expect(renderAnnotationsToFile).toHaveBeenCalledWith(
      'https://artifacts.example.com/original.png',
      annotationState,
      'image-published-annotated.png',
    );
  });

  it('uses the original attachment when publishing an attachment-backed annotation', async () => {
    const annotationState = {
      version: 1,
      baseWidth: 800,
      baseHeight: 600,
      shapes: [{ id: 'shape-1', type: 'rect', x: 10, y: 10, width: 100, height: 80, color: '#ef4444', strokeWidth: 4, rotation: 0 }],
    };
    await prepareDocsContentForPublish({
      type: 'doc',
      content: [{
        type: 'resizableImage',
        attrs: { src: '/rendered.png', sourceAttachmentId: 'original-attachment', annotationState },
      }],
    }, { uploadConfig });

    expect(renderAnnotationsToFile).toHaveBeenCalledWith(
      expect.stringContaining('/pm/attachments/original-attachment/content?proxy=1'),
      annotationState,
      'image-published-annotated.png',
    );
  });
});
