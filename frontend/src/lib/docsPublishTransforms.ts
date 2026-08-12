import type { JSONContent } from '@tiptap/react';
import { uploadEditorImage, type EditorUploadConfig } from '@/hooks/useEditorImageUpload';
import { mermaidSvgFile, renderMermaidSvg } from './mermaidRenderer';
import { excalidrawPngFile, normalizeExcalidrawScene } from './excalidrawRenderer';

export type PublishTransformContext = {
  uploadConfig: EditorUploadConfig;
  diagramTheme?: {
    brandColor?: string;
  };
};

export type PublishContentTransform = {
  name: string;
  appliesTo: (node: JSONContent) => boolean;
  transform: (node: JSONContent, ctx: PublishTransformContext) => Promise<JSONContent>;
};

function nodeText(node: JSONContent): string {
  if (typeof node.text === 'string') return node.text;
  return (node.content ?? []).map(nodeText).join('');
}

async function transformNode(
  node: JSONContent,
  ctx: PublishTransformContext,
  transforms: PublishContentTransform[],
): Promise<JSONContent> {
  const transform = transforms.find((candidate) => candidate.appliesTo(node));
  if (transform) {
    return transform.transform(node, ctx);
  }

  if (!Array.isArray(node.content) || node.content.length === 0) {
    return { ...node };
  }

  return {
    ...node,
    content: await Promise.all(node.content.map((child) => transformNode(child, ctx, transforms))),
  };
}

export const mermaidCodeBlockToImage: PublishContentTransform = {
  name: 'mermaid-code-block-to-image',
  appliesTo: (node) => node.type === 'codeBlock' && String(node.attrs?.language ?? '').toLowerCase() === 'mermaid',
  async transform(node, ctx) {
    const source = nodeText(node).trim();
    const timestamp = Date.now();
    const lightSvg = await renderMermaidSvg(source, 'light', { brandColor: ctx.diagramTheme?.brandColor });
    const darkSvg = await renderMermaidSvg(source, 'dark', { brandColor: ctx.diagramTheme?.brandColor });
    const lightUpload = await uploadEditorImage(
      mermaidSvgFile(lightSvg, `mermaid-${timestamp}-light.svg`),
      ctx.uploadConfig,
    );
    const darkUpload = await uploadEditorImage(
      mermaidSvgFile(darkSvg, `mermaid-${timestamp}-dark.svg`),
      ctx.uploadConfig,
    );

    return {
      type: 'resizableImage',
      attrs: {
        src: lightUpload.publicUrl,
        darkSrc: darkUpload.publicUrl,
        alt: 'Mermaid diagram',
        title: 'Mermaid diagram',
        width: '100%',
        height: 'auto',
        alignment: 'center',
        attachmentId: lightUpload.attachmentId,
        darkAttachmentId: darkUpload.attachmentId,
        publishedFrom: {
          type: 'codeBlock',
          attrs: { language: 'mermaid' },
          content: source ? [{ type: 'text', text: source }] : undefined,
        },
      },
    };
  },
};

export const excalidrawBlockToImage: PublishContentTransform = {
  name: 'excalidraw-block-to-image',
  appliesTo: (node) => node.type === 'excalidraw',
  async transform(node, ctx) {
    const title = typeof node.attrs?.title === 'string' && node.attrs.title.trim()
      ? node.attrs.title.trim()
      : 'Excalidraw drawing';
    const scene = normalizeExcalidrawScene(node.attrs?.scene);
    if (scene.elements.length === 0) {
      return {
        ...node,
        attrs: {
          ...node.attrs,
          title,
          scene,
        },
      };
    }
    const timestamp = Date.now();
    const lightUpload = await uploadEditorImage(
      await excalidrawPngFile(scene, `excalidraw-${timestamp}-light.png`, 'light'),
      ctx.uploadConfig,
    );
    const darkUpload = await uploadEditorImage(
      await excalidrawPngFile(scene, `excalidraw-${timestamp}-dark.png`, 'dark'),
      ctx.uploadConfig,
    );

    return {
      type: 'resizableImage',
      attrs: {
        src: lightUpload.publicUrl,
        darkSrc: darkUpload.publicUrl,
        alt: title,
        title,
        width: typeof node.attrs?.width === 'string' ? node.attrs.width : '100%',
        height: typeof node.attrs?.height === 'string' ? node.attrs.height : 'auto',
        alignment: typeof node.attrs?.alignment === 'string' ? node.attrs.alignment : 'center',
        attachmentId: lightUpload.attachmentId,
        darkAttachmentId: darkUpload.attachmentId,
        publishedFrom: {
          ...node,
          attrs: {
            ...node.attrs,
            title,
            scene,
          },
        },
      },
    };
  },
};

/**
 * Drops editor-only annotation data from published content.
 *
 * `annotationState` exists solely so an annotation can be re-opened and edited in the app; the
 * published image is already flattened, so carrying the state adds weight to public HTML, to
 * search indexing, and to embeddings for no reader benefit.
 *
 * `sourceAttachmentId` is dropped too, and that one matters: it points at the ORIGINAL,
 * un-annotated image. On a public help-center article it would hand anyone a link to the
 * pre-redaction version.
 */
export const stripImageAnnotationState: PublishContentTransform = {
  name: 'strip-image-annotation-state',
  appliesTo: (node) =>
    node.type === 'resizableImage' &&
    (node.attrs?.annotationState != null || node.attrs?.sourceAttachmentId != null),
  async transform(node) {
    const attrs = { ...(node.attrs ?? {}) };
    delete attrs.annotationState;
    delete attrs.sourceAttachmentId;
    return { ...node, attrs };
  },
};

export async function prepareDocsContentForPublish(
  content: JSONContent | null | undefined,
  ctx: PublishTransformContext,
  transforms: PublishContentTransform[] = [
    mermaidCodeBlockToImage,
    excalidrawBlockToImage,
    stripImageAnnotationState,
  ],
): Promise<JSONContent | undefined> {
  if (!content) return undefined;
  return transformNode(content, ctx, transforms);
}
