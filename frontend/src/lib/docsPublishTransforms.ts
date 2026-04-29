import type { JSONContent } from '@tiptap/react';
import { uploadEditorImage, type EditorUploadConfig } from '@/hooks/useEditorImageUpload';
import { mermaidSvgFile, renderMermaidSvg } from './mermaidRenderer';

export type PublishTransformContext = {
  uploadConfig: EditorUploadConfig;
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

  if (!node.content?.length) {
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
    const svg = await renderMermaidSvg(source);
    const upload = await uploadEditorImage(
      mermaidSvgFile(svg, `mermaid-${Date.now()}.svg`),
      ctx.uploadConfig,
    );

    return {
      type: 'resizableImage',
      attrs: {
        src: upload.publicUrl,
        alt: 'Mermaid diagram',
        title: 'Mermaid diagram',
        width: '100%',
        height: 'auto',
        alignment: 'center',
        attachmentId: upload.attachmentId,
        publishedFrom: {
          type: 'codeBlock',
          attrs: { language: 'mermaid' },
          content: source ? [{ type: 'text', text: source }] : undefined,
        },
      },
    };
  },
};

export async function prepareDocsContentForPublish(
  content: JSONContent | null | undefined,
  ctx: PublishTransformContext,
  transforms: PublishContentTransform[] = [mermaidCodeBlockToImage],
): Promise<JSONContent | undefined> {
  if (!content) return undefined;
  return transformNode(content, ctx, transforms);
}
