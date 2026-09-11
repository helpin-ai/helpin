import type { JSONContent } from '@tiptap/react';
import { uploadEditorImage, type EditorUploadConfig } from '@/hooks/useEditorImageUpload';
import { parseAnnotationState } from '@/components/docs/annotator/core/annotationTypes';
import { parseHelpinReference } from '@/lib/helpinReferences';
import { automationService } from '@/lib/services/automationService';
import { pmAttachmentService } from '@/lib/services/pmAttachmentService';
import { renderNwdiagSvg } from './nwdiagRenderer';
import { mermaidSvgFile, renderMermaidSvg } from './mermaidRenderer';
import { excalidrawPngFile, normalizeExcalidrawScene } from './excalidrawRenderer';
import { sanitizeSvg } from './svgRenderer';

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

export const svgCodeBlockToImage: PublishContentTransform = {
  name: 'svg-code-block-to-image',
  appliesTo: (node) => node.type === 'codeBlock' && String(node.attrs?.language ?? '').toLowerCase() === 'svg',
  async transform(node, ctx) {
    const source = nodeText(node).trim();
    const svg = sanitizeSvg(source);
    const upload = await uploadEditorImage(new File([svg], `svg-${Date.now()}.svg`, { type: 'image/svg+xml' }), ctx.uploadConfig);
    return { type: 'resizableImage', attrs: {
      src: upload.publicUrl, alt: 'SVG diagram', title: 'SVG diagram',
      width: '100%', height: 'auto', alignment: 'center', attachmentId: upload.attachmentId,
      publishedFrom: node,
    } };
  },
};

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

export const nwdiagCodeBlockToImage: PublishContentTransform = {
  name: 'nwdiag-code-block-to-image',
  appliesTo: (node) => node.type === 'codeBlock' && String(node.attrs?.language ?? '').toLowerCase() === 'nwdiag',
  async transform(node, ctx) {
    const source = nodeText(node).trim();
    const timestamp = Date.now();
    const lightSvg = await renderNwdiagSvg(source, 'light', { brandColor: ctx.diagramTheme?.brandColor });
    const darkSvg = await renderNwdiagSvg(source, 'dark', { brandColor: ctx.diagramTheme?.brandColor });
    const lightUpload = await uploadEditorImage(
      new File([lightSvg], `nwdiag-${timestamp}-light.svg`, { type: 'image/svg+xml' }),
      ctx.uploadConfig,
    );
    const darkUpload = await uploadEditorImage(
      new File([darkSvg], `nwdiag-${timestamp}-dark.svg`, { type: 'image/svg+xml' }),
      ctx.uploadConfig,
    );

    return {
      type: 'resizableImage',
      attrs: {
        src: lightUpload.publicUrl,
        darkSrc: darkUpload.publicUrl,
        alt: 'nwdiag diagram',
        title: 'nwdiag diagram',
        width: '100%',
        height: 'auto',
        alignment: 'center',
        attachmentId: lightUpload.attachmentId,
        darkAttachmentId: darkUpload.attachmentId,
        publishedFrom: {
          type: 'codeBlock',
          attrs: { language: 'nwdiag' },
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
 * Flattens editable annotations into a new public image and drops editor-only provenance.
 *
 * `annotationState` exists solely so an annotation can be re-opened and edited in the app.
 * Draft images normally already point at a flattened preview, but publishing renders again from
 * the stored original. This guarantees the public asset matches the editable state and avoids
 * depending on a stale preview produced by an older editor version.
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
  async transform(node, ctx) {
    const attrs = { ...(node.attrs ?? {}) };
    const annotationState = parseAnnotationState(attrs.annotationState);

    if (annotationState?.shapes.length) {
      let sourceUrl: string | null = null;
      const srcReference = parseHelpinReference(typeof attrs.src === 'string' ? attrs.src : undefined);
      const artifactId = typeof attrs.artifactId === 'string' && attrs.artifactId
        ? attrs.artifactId
        : srcReference?.type === 'artifacts'
          ? srcReference.id
          : null;

      // Artifact-backed legacy nodes sometimes point sourceAttachmentId at an older flattened
      // preview. The artifact is the authoritative original and must win or annotations would be
      // painted twice when the document is published.
      if (artifactId) {
        const response = await automationService.getArtifactContentURL(ctx.uploadConfig.workspaceId, artifactId);
        if (response.error || !response.data?.url) {
          throw new Error(response.error || 'Could not load the original annotated image for publishing');
        }
        sourceUrl = response.data.url;
      } else if (typeof attrs.sourceAttachmentId === 'string' && attrs.sourceAttachmentId) {
        sourceUrl = pmAttachmentService.proxiedContentUrl(attrs.sourceAttachmentId);
      }

      if (sourceUrl) {
        const { renderAnnotationsToFile } = await import(
          '@/components/docs/annotator/core/renderAnnotations'
        );
        const baseName = typeof attrs.alt === 'string' && attrs.alt.trim()
          ? attrs.alt.trim().replace(/[^a-z0-9_-]+/gi, '-')
          : 'image';
        const file = await renderAnnotationsToFile(
          sourceUrl,
          annotationState,
          `${baseName}-published-annotated.png`,
        );
        const upload = await uploadEditorImage(file, ctx.uploadConfig);
        attrs.src = upload.publicUrl;
        attrs.attachmentId = upload.attachmentId;
      }
    }

    delete attrs.annotationState;
    delete attrs.sourceAttachmentId;
    // These point at unannotated originals or alternates. Public rendering must use only the
    // freshly flattened asset in `src`, including in dark mode.
    delete attrs.artifactId;
    delete attrs.darkSrc;
    delete attrs.darkAttachmentId;
    return { ...node, attrs };
  },
};

export async function prepareDocsContentForPublish(
  content: JSONContent | null | undefined,
  ctx: PublishTransformContext,
  transforms: PublishContentTransform[] = [
    svgCodeBlockToImage,
    mermaidCodeBlockToImage,
    nwdiagCodeBlockToImage,
    excalidrawBlockToImage,
    stripImageAnnotationState,
  ],
): Promise<JSONContent | undefined> {
  if (!content) return undefined;
  return transformNode(content, ctx, transforms);
}
