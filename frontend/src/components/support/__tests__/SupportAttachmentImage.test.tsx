// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { SupportAttachmentGallery } from '../SupportAttachmentGallery';
import { fetchWithSessionAuth } from '@/lib/api';

vi.mock('@/lib/api', () => ({ API_BASE: 'https://api.example.com/api', fetchWithSessionAuth: vi.fn() }));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('support image recovery', () => {
  let container: HTMLDivElement;
  let root: ReturnType<typeof createRoot>;
  const attachment = { id: 'image', file_name: 'image.png', file_key: 'private/image.png', file_type: 'image/png', file_size: 5, url: 'https://storage.example.com/image.png?expired=1' };
  const revoke = vi.fn();

  beforeEach(async () => {
    vi.useFakeTimers();
    vi.clearAllMocks();
    vi.stubGlobal('URL', class extends URL {
      static createObjectURL = vi.fn(() => 'blob:recovered-image');
      static revokeObjectURL = revoke;
    });
    vi.mocked(fetchWithSessionAuth).mockResolvedValue({ ok: true, blob: async () => new Blob(['image'], { type: 'image/png' }) } as Response);
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
    await act(async () => root.render(<SupportAttachmentGallery workspaceId="workspace" conversationId="conversation" attachments={[attachment]} />));
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    vi.clearAllTimers(); vi.useRealTimers(); vi.unstubAllGlobals();
  });

  it('recovers a blocked thumbnail through the authorized API and shares it with the enlarged preview', async () => {
    const thumbnail = container.querySelector('img')!;
    await act(async () => thumbnail.dispatchEvent(new Event('error')));
    expect(fetchWithSessionAuth).toHaveBeenCalledWith('https://api.example.com/api', '/support/inbox/conversations/conversation/attachments/image/content?workspace_id=workspace', expect.objectContaining({ signal: expect.any(AbortSignal) }));
    expect(thumbnail.getAttribute('src')).toBe('blob:recovered-image');
    await act(async () => thumbnail.dispatchEvent(new Event('load')));
    await act(async () => container.querySelector<HTMLButtonElement>('button[aria-label="Preview image.png"]')!.click());
    const enlarged = document.querySelector('[data-testid="support-attachment-lightbox"] img')!;
    await act(async () => enlarged.dispatchEvent(new Event('error')));
    expect(enlarged.getAttribute('src')).toBe('blob:recovered-image');
    expect(fetchWithSessionAuth).toHaveBeenCalledTimes(1);
    await act(async () => root.unmount());
    expect(revoke).toHaveBeenCalledWith('blob:recovered-image');
  });

  it('recovers a storage request that stalls without firing an error', async () => {
    await act(async () => vi.advanceTimersByTime(8_000));
    expect(fetchWithSessionAuth).toHaveBeenCalledTimes(1);
    expect(container.querySelector('img')?.getAttribute('src')).toBe('blob:recovered-image');
  });

  it('offers retry in the enlarged preview when the API also fails', async () => {
    vi.mocked(fetchWithSessionAuth).mockResolvedValue({ ok: false, status: 503 } as Response);
    await act(async () => container.querySelector<HTMLButtonElement>('button[aria-label="Preview image.png"]')!.click());
    const preview = document.querySelector('[data-testid="support-attachment-lightbox"]')!;
    await act(async () => preview.querySelector('img')!.dispatchEvent(new Event('error')));
    expect(preview.textContent).toContain('Could not load image');
    const retry = Array.from(preview.querySelectorAll('button')).find(button => button.textContent === 'Retry');
    expect(retry).toBeTruthy();
    vi.mocked(fetchWithSessionAuth).mockResolvedValue({ ok: true, blob: async () => new Blob(['image'], { type: 'image/png' }) } as Response);
    await act(async () => retry!.click());
    await act(async () => preview.querySelector('img')!.dispatchEvent(new Event('error')));
    expect(preview.querySelector('img')?.getAttribute('src')).toBe('blob:recovered-image');
  });

  it('keeps an image visible if the original request finishes before a failed fallback', async () => {
    let reject!: (error: Error) => void;
    vi.mocked(fetchWithSessionAuth).mockReturnValue(new Promise((_resolve, rejectPromise) => { reject = rejectPromise; }));
    await act(async () => vi.advanceTimersByTime(8_000));
    const image = container.querySelector('img')!;
    await act(async () => image.dispatchEvent(new Event('load')));
    await act(async () => reject(new Error('Network error')));
    expect(image.className).not.toContain('invisible');
    expect(container.textContent).not.toContain('Could not load image');
  });

  it('keeps offscreen thumbnails deferred', async () => {
    await act(async () => root.unmount());
    const observe = vi.fn();
    vi.stubGlobal('IntersectionObserver', class { observe = observe; disconnect = vi.fn(); });
    root = createRoot(container);
    await act(async () => root.render(<SupportAttachmentGallery workspaceId="workspace" conversationId="conversation" attachments={[attachment]} />));
    await act(async () => vi.advanceTimersByTime(8_000));
    expect(observe).toHaveBeenCalled();
    expect(container.querySelector('img')?.getAttribute('src')).toBeNull();
    expect(fetchWithSessionAuth).not.toHaveBeenCalled();
  });
});
