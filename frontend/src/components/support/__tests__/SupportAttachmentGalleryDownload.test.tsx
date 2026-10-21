// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { toast } from 'sonner';
import { SupportAttachmentGallery } from '../SupportAttachmentGallery';

vi.mock('sonner', () => ({ toast: { error: vi.fn() } }));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const attachment = {
  id: 'image', file_key: 'image.png', file_name: 'Screenshot.png',
  file_type: 'image/png', file_size: 128, url: 'https://storage.example.com/image.png?signed=preview',
};

describe('support attachment downloads', () => {
  let container: HTMLDivElement;
  let root: ReturnType<typeof createRoot>;
  let savedFiles: { href: string; filename: string; target: string }[];
  const fetchMock = vi.fn();
  const createObjectURL = vi.fn(() => 'blob:saved-image');
  const revokeObjectURL = vi.fn();

  beforeEach(async () => {
    vi.useFakeTimers();
    vi.clearAllMocks();
    vi.stubGlobal('fetch', fetchMock);
    vi.stubGlobal('URL', class extends URL {
      static createObjectURL = createObjectURL;
      static revokeObjectURL = revokeObjectURL;
    });
    savedFiles = [];
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
      savedFiles.push({ href: this.href, filename: this.download, target: this.target });
    });
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
    await act(async () => root.render(<SupportAttachmentGallery attachments={[attachment]} />));
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    vi.clearAllTimers();
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  async function openPreview(surface: 'hover' | 'lightbox') {
    const thumbnail = container.querySelector<HTMLButtonElement>('button[aria-label="Preview Screenshot.png"]')!;
    await act(async () => {
      if (surface === 'lightbox') thumbnail.click();
      else thumbnail.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }));
    });
    const preview = document.querySelector(`[data-testid="support-attachment-${surface === 'hover' ? 'hover-preview' : 'lightbox'}"]`)!;
    const download = preview.querySelector<HTMLButtonElement>('button[aria-label="Download image attachment"]');
    expect(download).not.toBeNull();
    return { preview, download: download! };
  }

  it.each(['hover', 'lightbox'] as const)('saves the image from the %s preview without opening the storage URL', async (surface) => {
    const blob = new Blob(['image bytes'], { type: 'image/png' });
    fetchMock.mockResolvedValue({ ok: true, blob: async () => blob });
    const { preview, download } = await openPreview(surface);
    await act(async () => download.click());

    expect(fetchMock).toHaveBeenCalledWith(attachment.url, expect.objectContaining({ credentials: 'omit', signal: expect.any(AbortSignal) }));
    expect(createObjectURL).toHaveBeenCalledWith(blob);
    expect(savedFiles).toEqual([{ href: 'blob:saved-image', filename: 'Screenshot.png', target: '' }]);
    expect(preview.querySelector('img')?.getAttribute('src')).toBe(attachment.url);
    expect(preview.isConnected).toBe(true);
    expect(toast.error).not.toHaveBeenCalled();
    expect(revokeObjectURL).not.toHaveBeenCalled();
    await act(async () => vi.advanceTimersByTime(60_000));
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:saved-image');
  });

  it('does not save storage error responses as images and allows another attempt', async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 403 });
    const { preview, download } = await openPreview('lightbox');
    await act(async () => download.click());
    expect(savedFiles).toEqual([]);
    expect(createObjectURL).not.toHaveBeenCalled();
    expect(toast.error).toHaveBeenCalledWith('Could not download the attachment. Refresh the conversation and try again.');
    expect(download.disabled).toBe(false);
    expect(preview.isConnected).toBe(true);
  });

  it('prevents duplicate downloads while the file is loading', async () => {
    let finish!: (response: unknown) => void;
    fetchMock.mockReturnValue(new Promise(resolve => { finish = resolve; }));
    const { download } = await openPreview('lightbox');
    await act(async () => { download.click(); download.click(); });
    expect(download.disabled).toBe(true);
    expect(download.getAttribute('aria-busy')).toBe('true');
    expect(fetchMock).toHaveBeenCalledTimes(1);
    await act(async () => finish({ ok: true, blob: async () => new Blob(['image']) }));
    expect(download.disabled).toBe(false);
    expect(savedFiles).toHaveLength(1);
  });

  it('stops a stalled download and keeps the preview usable', async () => {
    fetchMock.mockImplementation((_url: string, { signal }: RequestInit) => new Promise((_resolve, reject) => {
      signal!.addEventListener('abort', () => reject(new DOMException('Timed out', 'AbortError')));
    }));
    const { preview, download } = await openPreview('lightbox');
    await act(async () => download.click());
    await act(async () => vi.advanceTimersByTime(30_000));
    expect(download.disabled).toBe(false);
    expect(savedFiles).toEqual([]);
    expect(toast.error).toHaveBeenCalledWith('Download timed out. Please try again.');
    expect(preview.isConnected).toBe(true);
  });
});
