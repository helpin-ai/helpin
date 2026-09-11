// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { getSchema, Node as TiptapNode } from '@tiptap/core';
import StarterKit from '@tiptap/starter-kit';
import { createDocsExport, docsExportDoc, docsExportHtml, docsExportMdx, prepareDocsExportHtml } from '../docsExport';
import { renderMermaidSvg } from '../mermaidRenderer';
import { fetchWithSessionAuth } from '../api';
import { automationService } from '../services/automationService';

vi.mock('../mermaidRenderer', () => ({ renderMermaidSvg: vi.fn(async () => '<svg viewBox="0 0 200 100"/>') }));
vi.mock('../nwdiagRenderer', () => ({ renderNwdiagSvg: vi.fn(async () => '<svg viewBox="0 0 100 200"/>') }));
vi.mock('../excalidrawRenderer', () => ({ exportExcalidrawPngBlob: vi.fn(async () => new Blob(['drawing'], { type: 'image/png' })) }));
vi.mock('../api', () => ({ API_BASE: 'https://api.example.com/api', fetchWithSessionAuth: vi.fn() }));
vi.mock('../services/automationService', () => ({ automationService: { getArtifactContentURL: vi.fn() } }));

const image = TiptapNode.create({
  name: 'resizableImage', group: 'block', atom: true,
  addAttributes: () => ({ src: {}, alt: {}, title: {}, width: {}, height: {} }),
  renderHTML: ({ HTMLAttributes }) => ['img', HTMLAttributes],
});
const drawing = TiptapNode.create({ name: 'excalidraw', group: 'block', atom: true, renderHTML: () => ['div'] });
const htmlBlock = TiptapNode.create({
  name: 'htmlBlock', group: 'block', atom: true,
  addAttributes: () => ({ html: {} }),
  renderHTML: ({ node }) => {
    const div = document.createElement('div');
    div.innerHTML = node.attrs.html;
    return div;
  },
});
const schema = getSchema([StarterKit, image, drawing, htmlBlock]);
const png = 'data:image/png;base64,cG5n';
const response = () => ({ ok: true, blob: async () => new Blob(['photo'], { type: 'image/png' }) });

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn(async () => response()));
  vi.mocked(fetchWithSessionAuth).mockResolvedValue(response() as Response);
  vi.stubGlobal('Image', class {
    naturalWidth = 100;
    naturalHeight = 80;
    onload?: () => void;
    set src(_src: string) { queueMicrotask(() => this.onload?.()); }
  });
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({ fillRect: vi.fn(), drawImage: vi.fn() } as unknown as CanvasRenderingContext2D);
  vi.spyOn(HTMLCanvasElement.prototype, 'toDataURL').mockReturnValue(png);
});
afterEach(() => { vi.restoreAllMocks(); vi.clearAllMocks(); vi.unstubAllGlobals(); });

describe('document image exports', () => {
  it('renders nested diagrams and drawings without mutating source or ordinary code', async () => {
    const content = { type: 'doc', content: [
      { type: 'blockquote', content: ['mermaid', 'nwdiag'].map((language) => ({ type: 'codeBlock', attrs: { language }, content: [{ type: 'text', text: 'diagram source' }] })) },
      { type: 'excalidraw', attrs: { scene: { elements: [{ type: 'rectangle' }] } } },
      { type: 'codeBlock', attrs: { language: 'js' }, content: [{ type: 'text', text: 'const x = "<div>";' }] },
    ] };
    const before = JSON.stringify(content);
    const html = await prepareDocsExportHtml(content, schema, 'ws');
    expect(html.match(/<img /g)).toHaveLength(3);
    expect(html).not.toContain('diagram source');
    expect(html).toContain('const x = "&lt;div&gt;";');
    expect(renderMermaidSvg).toHaveBeenCalledWith('diagram source', 'light', { htmlLabels: false });
    expect(JSON.stringify(content)).toBe(before);
  });

  it('fails clearly for an invalid diagram instead of exporting missing content', async () => {
    vi.mocked(renderMermaidSvg).mockRejectedValueOnce(new Error('invalid syntax'));
    await expect(prepareDocsExportHtml({ type: 'doc', content: [{ type: 'codeBlock', attrs: { language: 'mermaid' } }] }, schema, 'ws'))
      .rejects.toThrow('Could not export a mermaid diagram');
  });

  it('embeds protected flattened previews without exposing annotation originals or credentials', async () => {
    const html = await prepareDocsExportHtml({ type: 'doc', content: [
      { type: 'resizableImage', attrs: { src: 'https://old.example.com/expired', attachmentId: 'flattened', sourceAttachmentId: 'secret-original', artifactId: 'original-artifact', annotationState: { shapes: [{}] } } },
      { type: 'resizableImage', attrs: { src: 'https://cdn.example.com/photo.png' } },
    ] }, schema, 'ws');
    expect(html.match(/data:image\/png;base64/g)).toHaveLength(2);
    expect(html).not.toMatch(/secret-original|artifact|annotation|attachment|expired/);
    expect(fetchWithSessionAuth).toHaveBeenCalledWith('https://api.example.com/api', '/pm/attachments/flattened/content?proxy=1', expect.any(Object));
    expect(fetch).toHaveBeenCalledWith('https://cdn.example.com/photo.png', expect.objectContaining({ credentials: 'omit' }));
    expect(automationService.getArtifactContentURL).not.toHaveBeenCalled();
  });

  it('resolves artifact images and deduplicates repeated image downloads', async () => {
    vi.mocked(automationService.getArtifactContentURL).mockResolvedValue({ data: { url: 'https://cdn.example.com/a.png', expires_at: '' }, error: null });
    const html = await prepareDocsExportHtml({ type: 'doc', content: [
      { type: 'resizableImage', attrs: { src: 'helpin://artifacts/a' } },
      { type: 'resizableImage', attrs: { src: 'https://cdn.example.com/a.png' } },
    ] }, schema, 'ws');
    expect(html.match(/<img /g)).toHaveLength(2);
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(automationService.getArtifactContentURL).toHaveBeenCalledWith('ws', 'a');
  });

  it('embeds HTML-block images and strips executable HTML and editor metadata', async () => {
    const html = await prepareDocsExportHtml({ type: 'doc', content: [{ type: 'htmlBlock', attrs: { html: '<p data-secret="hidden" style="background: url(https://evil.example/x)">Hello</p><img src="https://cdn.example.com/a.png" onerror="alert(1)"><script>alert(1)</script>' } }] }, schema, 'ws');
    expect(html).toContain('data:image/png;base64,');
    expect(html).not.toMatch(/data-secret|script|onerror|evil\.example/);
  });

  it('rejects unavailable images', async () => {
    vi.mocked(fetch).mockResolvedValueOnce({ ok: false } as Response);
    await expect(prepareDocsExportHtml({ type: 'doc', content: [{ type: 'resizableImage', attrs: { src: 'https://cdn.example.com/gone.png' } }] }, schema, 'ws')).rejects.toThrow('Could not download an image');
  });

  it('converts WebP images to PNG for Word compatibility', async () => {
    vi.mocked(fetch).mockResolvedValueOnce({ ok: true, blob: async () => new Blob(['webp'], { type: 'image/webp' }) } as Response);
    const html = await prepareDocsExportHtml({ type: 'doc', content: [{ type: 'resizableImage', attrs: { src: 'https://cdn.example.com/a.webp' } }] }, schema, 'ws');
    expect(html).toContain(png);
    expect(html).not.toContain('image/webp');
  });

  it('retains checked and unchecked task markers in static output', async () => {
    const html = await prepareDocsExportHtml({ type: 'doc', content: [{ type: 'htmlBlock', attrs: { html: '<ul><li><input type="checkbox" checked>Done</li><li><input type="checkbox">Pending</li></ul>' } }] }, schema, 'ws');
    expect(html).toContain('☑ ');
    expect(html).toContain('☐ ');
    expect(html).not.toContain('<input');
  });
});

describe('export containers', () => {
  it('makes standalone HTML with escaped Unicode titles and embedded images', () => {
    const html = docsExportHtml(`<p>Hello</p><img src="${png}">`, 'Résumé <draft>');
    expect(html).toContain('<title>Résumé &lt;draft&gt;</title>');
    expect(html).toContain(png);
    expect(html).toContain('charset="utf-8"');
    expect(createDocsExport('', '', 'html').type).toBe('text/html;charset=utf-8');
  });

  it('produces JSX-compatible MDX for styles, void tags, tables and literal expressions', () => {
    const mdx = docsExportMdx(`<p class="note" style="text-align: center">{danger()} &amp; &lt;Widget&gt;</p><br><img src="${png}"><table><tbody><tr><td colspan="2">x</td></tr></tbody></table><pre><code>a\nb</code></pre>`);
    expect(mdx).toContain('className="note" style={{"textAlign":"center"}}');
    expect(mdx).toContain('{"{danger()} & <Widget>"}');
    expect(mdx).toContain('<br />');
    expect(mdx).toContain(`src="${png}" />`);
    expect(mdx).toContain('colSpan="2"');
    expect(mdx).toContain('{"a\\nb"}');
    expect(createDocsExport('', '', 'mdx').type).toBe('text/mdx;charset=utf-8');
  });

  it('keeps prose readable and prevents literal text becoming MDX code or Markdown blocks', () => {
    const mdx = docsExportMdx('<h2>Heading</h2><p>Hello {name} &lt;Component&gt;</p><p>export const secret = 1</p><p>---</p><p>1. Literal</p>');
    expect(mdx).toContain('## Heading');
    expect(mdx).toContain('Hello \\{name\\} &lt;Component&gt;');
    expect(mdx).toContain('<p>{"export const secret = 1"}</p>');
    expect(mdx).toContain('\\---');
    expect(mdx).toContain('1\\. Literal');
  });

  it('packages Word HTML and deduplicated image MIME parts using UTF-8 base64', () => {
    const doc = docsExportDoc(`<p>Résumé 🎉</p><img src="${png}"><img src="${png}">`, 'Résumé');
    expect(doc).toContain('Content-Type: multipart/related;');
    expect(doc.match(/Content-Type: image\/png/g)).toHaveLength(1);
    const htmlPart = doc.split('Content-Location: file:///helpin-export/document.html\r\n\r\n')[1].split('\r\n--')[0];
    const html = new TextDecoder().decode(Uint8Array.from(atob(htmlPart.replace(/\r\n/g, '')), (c) => c.charCodeAt(0)));
    expect(html).toContain('Résumé 🎉');
    expect(html).not.toContain('data:image');
    expect(html.match(/src="file:\/\/\/helpin-export\/image-1"/g)).toHaveLength(2);
    expect(createDocsExport('', '', 'doc').type).toBe('application/msword');
  });
});
