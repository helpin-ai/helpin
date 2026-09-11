import type { JSONContent } from '@tiptap/core';
import { DOMSerializer, type Schema } from '@tiptap/pm/model';
import { sanitizeHtml } from '@/components/docs/htmlSanitizer';
import { API_BASE, fetchWithSessionAuth } from './api';
import { parseHelpinReference } from './helpinReferences';
import { automationService } from './services/automationService';
import { pmAttachmentService } from './services/pmAttachmentService';
import { renderMermaidSvg } from './mermaidRenderer';
import { renderNwdiagSvg } from './nwdiagRenderer';
import { exportExcalidrawPngBlob } from './excalidrawRenderer';

export type DocsExportFormat = 'doc' | 'html' | 'mdx';

function escapeHtml(text: string): string {
  return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

function blobDataUrl(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result));
    reader.onerror = () => reject(new Error('Could not read an export image.'));
    reader.readAsDataURL(blob);
  });
}

/** Raster images work in Word and do not depend on SVG/foreignObject support. */
export async function svgToPngDataUrl(svg: string): Promise<string> {
  const root = new DOMParser().parseFromString(svg, 'image/svg+xml').documentElement;
  const viewBox = root.getAttribute('viewBox')?.trim().split(/[\s,]+/).map(Number);
  const width = viewBox?.[2] || parseFloat(root.getAttribute('width') || '') || 1024;
  const height = viewBox?.[3] || parseFloat(root.getAttribute('height') || '') || 768;
  if (!(width > 0 && height > 0 && Number.isFinite(width) && Number.isFinite(height))) {
    throw new Error('The diagram has invalid dimensions.');
  }
  root.setAttribute('width', String(width));
  root.setAttribute('height', String(height));
  const src = await blobDataUrl(new Blob([new XMLSerializer().serializeToString(root)], { type: 'image/svg+xml' }));
  return imageToPngDataUrl(src, width, height);
}

async function imageToPngDataUrl(src: string, width?: number, height?: number): Promise<string> {
  const img = new Image();
  await new Promise<void>((resolve, reject) => {
    img.onload = () => resolve();
    img.onerror = () => reject(new Error('Could not render an export image.'));
    img.src = src;
  });
  width ??= img.naturalWidth;
  height ??= img.naturalHeight;
  const canvas = document.createElement('canvas');
  const scale = Math.min(2, 4096 / Math.max(width, height));
  canvas.width = Math.max(1, Math.ceil(width * scale));
  canvas.height = Math.max(1, Math.ceil(height * scale));
  const context = canvas.getContext('2d');
  if (!context) throw new Error('Image export is unavailable in this browser.');
  context.fillStyle = '#ffffff';
  context.fillRect(0, 0, canvas.width, canvas.height);
  context.drawImage(img, 0, 0, canvas.width, canvas.height);
  return canvas.toDataURL('image/png');
}

async function embedImage(src: string): Promise<string> {
  const url = new URL(src, window.location.href);
  if (!['http:', 'https:', 'data:', 'blob:'].includes(url.protocol)) {
    throw new Error('An image has an unsupported address.');
  }
  const apiBase = new URL(API_BASE, window.location.href);
  // Only send session credentials to our own API, never to an external image host.
  const apiPath = apiBase.pathname.replace(/\/$/, '');
  const isApi = url.origin === apiBase.origin && url.pathname.startsWith(`${apiPath}/`);
  if (isApi && /\/pm\/attachments\/[^/]+\/content$/.test(url.pathname)) url.searchParams.set('proxy', '1');
  const init = { signal: AbortSignal.timeout(30_000) };
  const response = isApi
    ? await fetchWithSessionAuth(API_BASE, `${url.pathname.slice(apiPath.length)}${url.search}`, init)
    : await fetch(url.href, { ...init, credentials: 'omit' });
  if (!response.ok) throw new Error('Could not download an image. Check that it is still accessible.');
  const blob = await response.blob();
  if (blob.type.split(';')[0] === 'image/svg+xml') return svgToPngDataUrl(await blob.text());
  if (/^image\/(webp|avif|bmp)(;|$)/i.test(blob.type)) return imageToPngDataUrl(await blobDataUrl(blob));
  if (!/^image\/(png|jpe?g|gif|webp)(;|$)/i.test(blob.type)) {
    throw new Error('An image returned an unsupported file type.');
  }
  return blobDataUrl(blob);
}

/** Prepare a detached snapshot; exporting never edits the document or uploads assets. */
export async function prepareDocsExportHtml(content: JSONContent, schema: Schema, workspaceId?: string): Promise<string> {
  const cache = new Map<string, Promise<string>>();
  const cachedImage = (src: string) => {
    let result = cache.get(src);
    if (!result) { result = embedImage(src); cache.set(src, result); }
    return result;
  };
  async function transform(node: JSONContent): Promise<JSONContent> {
    const language = String(node.attrs?.language ?? '').toLowerCase();
    let diagram: string | undefined;
    if (node.type === 'codeBlock' && ['mermaid', 'nwdiag'].includes(language)) {
      const source = (node.content ?? []).map((child) => child.text ?? '').join('');
      try {
        const svg = language === 'mermaid'
          ? await renderMermaidSvg(source, 'light', { htmlLabels: false })
          : await renderNwdiagSvg(source, 'light');
        diagram = await svgToPngDataUrl(svg);
      } catch {
        throw new Error(`Could not export a ${language} diagram. Check its source and try again.`);
      }
    } else if (node.type === 'excalidraw') {
      try {
        diagram = await blobDataUrl(await exportExcalidrawPngBlob(node.attrs?.scene, 'light'));
      } catch {
        throw new Error('Could not export an Excalidraw drawing. Check that it is not empty and try again.');
      }
    }
    if (diagram) {
      return { type: 'resizableImage', attrs: { src: diagram, alt: node.attrs?.title || `${language || 'Excalidraw'} diagram`, width: '100%', height: 'auto' } };
    }
    if (node.type === 'resizableImage' || node.type === 'image') {
      const attrs = node.attrs ?? {};
      let src = String(attrs.src ?? '');
      const reference = parseHelpinReference(src);
      // The attachment/src is the flattened preview for annotated images. Never export
      // sourceAttachmentId or substitute an unannotated artifact for that preview.
      if (attrs.attachmentId) src = pmAttachmentService.proxiedContentUrl(String(attrs.attachmentId));
      else if (reference?.type === 'artifacts' || (attrs.artifactId && !attrs.annotationState)) {
        if (attrs.annotationState) throw new Error('Save the annotated image before exporting.');
        if (!workspaceId) throw new Error('Open this document in its workspace to export artifact images.');
        const result = await automationService.getArtifactContentURL(workspaceId, reference?.id ?? String(attrs.artifactId));
        if (result.error || !result.data?.url) throw new Error('Could not load an image artifact for export.');
        src = result.data.url;
      }
      if (!src) throw new Error('An image is missing its source.');
      return { type: node.type, attrs: {
        src: await cachedImage(src), alt: attrs.alt ?? '', title: attrs.title ?? '',
        width: attrs.width, height: attrs.height,
      } };
    }
    const transformed = { ...node };
    if (node.type === 'htmlBlock') transformed.attrs = { ...node.attrs, renderMode: 'inline' };
    if (node.content) {
      transformed.content = [];
      for (const child of node.content) transformed.content.push(await transform(child));
    }
    return transformed;
  }
  const snapshot = await transform(content);
  const container = document.createElement('div');
  container.appendChild(DOMSerializer.fromSchema(schema).serializeFragment(schema.nodeFromJSON(snapshot).content));
  // Include images inside HTML blocks, and remove all editor-only provenance attributes.
  for (const img of container.querySelectorAll('img')) {
    const src = img.getAttribute('src');
    if (!src) throw new Error('An image is missing its source.');
    if (!/^data:image\/(png|jpeg|gif);base64,/i.test(src)) img.src = await cachedImage(src);
  }
  for (const checkbox of container.querySelectorAll<HTMLInputElement>('input[type="checkbox"]')) {
    const marker = document.createElement('span');
    marker.textContent = checkbox.checked || checkbox.hasAttribute('checked') ? '☑ ' : '☐ ';
    checkbox.replaceWith(marker);
  }
  container.innerHTML = sanitizeHtml(container.innerHTML);
  for (const element of container.querySelectorAll('*')) {
    for (const attr of Array.from(element.attributes)) {
      if (attr.name.startsWith('data-')) element.removeAttribute(attr.name);
    }
    if (element instanceof HTMLAnchorElement && element.getAttribute('href')?.startsWith('/')) {
      element.href = new URL(element.getAttribute('href')!, window.location.href).href;
    }
  }
  return container.innerHTML;
}

export function docsExportHtml(body: string, title: string): string {
  return `<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>${escapeHtml(title || 'Document')}</title>
<style>
body { font-family: Arial, sans-serif; font-size: 11pt; line-height: 1.6; color: #1a1a1a; max-width: 800px; margin: 32px auto; padding: 0 24px; overflow-wrap: anywhere; }
h1,h2,h3,h4,h5,h6 { line-height: 1.25; margin: 1.2em 0 .5em; }
p { margin: 0 0 8pt; } ul,ol { padding-left: 24px; }
blockquote { border-left: 3px solid #ccc; padding-left: 12px; margin-left: 0; color: #555; }
code,pre { font-family: Consolas, monospace; background: #f4f4f4; } pre { padding: 12px; white-space: pre-wrap; }
a { color: #1a73e8; } img { max-width: 100%; height: auto; }
table { border-collapse: collapse; max-width: 100%; } th,td { border: 1px solid #ccc; padding: 6px 10px; }
</style></head><body>${body}</body></html>`;
}

/** Use Markdown for prose and standard JSX for rich blocks, with no custom components. */
export function docsExportMdx(body: string): string {
  const container = document.createElement('div');
  container.innerHTML = body;
  const voidTags = new Set(['br', 'hr', 'img', 'wbr', 'col']);
  const names: Record<string, string> = { class: 'className', colspan: 'colSpan', rowspan: 'rowSpan' };
  function jsx(node: Node): string {
    if (node.nodeType === Node.TEXT_NODE) {
      // Text expressions preserve whitespace in code and cannot execute document text.
      return `{${JSON.stringify(node.textContent ?? '')}}`;
    }
    if (!(node instanceof HTMLElement)) return '';
    const tag = node.tagName.toLowerCase();
    const attrs = Array.from(node.attributes).map(({ name, value }) => {
      if (name === 'style') {
        const style: Record<string, string> = {};
        for (const property of Array.from(node.style)) {
          style[property.replace(/-([a-z])/g, (_, letter: string) => letter.toUpperCase())] = node.style.getPropertyValue(property);
        }
        return ` style={${JSON.stringify(style)}}`;
      }
      return ` ${names[name] ?? name}="${escapeHtml(value)}"`;
    }).join('');
    if (voidTags.has(tag)) return `<${tag}${attrs} />`;
    return `<${tag}${attrs}>${Array.from(node.childNodes).map(jsx).join('')}</${tag}>`;
  }
  function markdownText(value: string): string {
    return value.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      .replace(/[\\`*_[\]{}#!|~]/g, '\\$&').replace(/^(\s*\d+)\./gm, '$1\\.')
      .replace(/^(\s*)([-+])/gm, '$1\\$2');
  }
  function block(node: Node): string {
    if (node instanceof HTMLElement) {
      const tag = node.tagName.toLowerCase();
      // Rich inline markup stays JSX so styles and nested marks retain their meaning.
      if (!node.attributes.length && !node.children.length && /^(p|h[1-6])$/.test(tag)) {
        const text = markdownText(node.textContent ?? '');
        // MDX treats import/export at the beginning of a line as JavaScript.
        if (/^(\s|(import|export)\s)/m.test(text) || /^[=-]+$/m.test(text)) return jsx(node);
        return tag === 'p' ? text : `${'#'.repeat(Number(tag[1]))} ${text}`;
      }
    }
    return jsx(node);
  }
  return `${Array.from(container.childNodes).map(block).join('\n\n')}\n`;
}

function base64Utf8(value: string): string {
  const bytes = new TextEncoder().encode(value);
  let binary = '';
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary);
}

/** Word's legacy HTML import reads MIME image parts; data URLs are unreliable there. */
export function docsExportDoc(body: string, title: string): string {
  const boundary = `helpin-${crypto.randomUUID()}`;
  const images = new Map<string, string>();
  // Keep MIME-only file URLs inert while constructing the archive in the browser.
  const container = document.createElement('template');
  container.innerHTML = body;
  for (const img of container.content.querySelectorAll('img')) {
    const src = img.getAttribute('src')!;
    let location = images.get(src);
    if (!location) { location = `file:///helpin-export/image-${images.size + 1}`; images.set(src, location); }
    img.src = location;
  }
  const wrap = (base64: string) => base64.match(/.{1,76}/g)?.join('\r\n') ?? '';
  const parts = [
    'MIME-Version: 1.0',
    `Content-Type: multipart/related; boundary="${boundary}"; type="text/html"`,
    '', `--${boundary}`, 'Content-Type: text/html; charset="utf-8"',
    'Content-Transfer-Encoding: base64', 'Content-Location: file:///helpin-export/document.html', '',
    wrap(base64Utf8(docsExportHtml(container.innerHTML, title))),
  ];
  for (const [src, location] of images) {
    const match = /^data:(image\/[a-z0-9.+-]+);base64,([a-z0-9+/=]+)$/i.exec(src);
    if (!match) throw new Error('An image could not be embedded in the Word export.');
    parts.push(`--${boundary}`, `Content-Type: ${match[1]}`, 'Content-Transfer-Encoding: base64', `Content-Location: ${location}`, '', wrap(match[2]));
  }
  parts.push(`--${boundary}--`, '');
  return parts.join('\r\n');
}

export function createDocsExport(body: string, title: string, format: DocsExportFormat): Blob {
  if (format === 'doc') return new Blob([docsExportDoc(body, title)], { type: 'application/msword' });
  if (format === 'mdx') return new Blob([docsExportMdx(body)], { type: 'text/mdx;charset=utf-8' });
  return new Blob([docsExportHtml(body, title)], { type: 'text/html;charset=utf-8' });
}
