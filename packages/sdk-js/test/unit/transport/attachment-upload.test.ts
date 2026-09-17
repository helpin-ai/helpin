import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { uploadAttachment } from '../../../src/transport/attachment-upload';

class UploadXHR {
  static current: UploadXHR;
  status = 200;
  timeout = 0;
  upload = { onprogress: null as ((event: ProgressEvent) => void) | null };
  onload: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onabort: (() => void) | null = null;
  ontimeout: (() => void) | null = null;
  open = vi.fn();
  setRequestHeader = vi.fn();
  send = vi.fn();
  abort = vi.fn(() => this.onabort?.());
  constructor() { UploadXHR.current = this; }
}
const file = new File(['video'], 'recording.mp4', { type: 'video/mp4' });
const initialized = { attachment: { id: 'att-1' }, upload_url: 'https://storage.test/signed', public_url: 'https://cdn.test/video.mp4' };
let fetchMock: ReturnType<typeof vi.fn>;
const start = (options = {}) => uploadAttachment('api.test', 'token', file, options);
const storage = async () => { await vi.waitFor(() => expect(UploadXHR.current?.send).toHaveBeenCalled()); return UploadXHR.current; };
beforeEach(() => {
  UploadXHR.current = undefined as unknown as UploadXHR;
  fetchMock = vi.fn().mockResolvedValueOnce({ ok: true, json: async () => initialized }).mockResolvedValue({ ok: true });
  vi.stubGlobal('fetch', fetchMock);
  vi.stubGlobal('XMLHttpRequest', UploadXHR);
});
afterEach(() => { vi.unstubAllGlobals(); vi.useRealTimers(); });

describe('attachment upload transport', () => {
  it('uses direct storage PUT byte progress and confirms before completion', async () => {
    const onProgress = vi.fn();
    const result = start({ onProgress });
    const xhr = await storage();
    expect(xhr.open).toHaveBeenCalledWith('PUT', initialized.upload_url, true);
    expect(xhr.send).toHaveBeenCalledWith(file);
    expect(xhr.setRequestHeader.mock.calls).toEqual([['Content-Type', 'video/mp4']]);
    xhr.upload.onprogress?.({ lengthComputable: true, loaded: 2, total: 5 } as ProgressEvent);
    expect(onProgress).toHaveBeenLastCalledWith(40);
    xhr.upload.onprogress?.({ lengthComputable: true, loaded: 5, total: 5 } as ProgressEvent);
    expect(onProgress).toHaveBeenLastCalledWith(99);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    xhr.onload?.();
    await expect(result).resolves.toEqual({ attachmentId: 'att-1', url: initialized.public_url });
    expect(fetchMock).toHaveBeenNthCalledWith(2, 'https://api.test/widget/support/attachments/att-1/confirm', expect.objectContaining({ method: 'PATCH', signal: expect.any(AbortSignal) }));
    expect(onProgress).toHaveBeenLastCalledWith(100);
  });
  it.each([undefined, ''])('confirms private storage uploads without a public URL (%s)', async public_url => {
    fetchMock.mockReset().mockResolvedValueOnce({ ok: true, json: async () => ({ ...initialized, public_url }) }).mockResolvedValue({ ok: true });
    const result = start();
    const xhr = await storage();
    expect(xhr.setRequestHeader.mock.calls).toEqual([['Content-Type', 'video/mp4']]);
    xhr.onload?.();
    await expect(result).resolves.toEqual({ attachmentId: 'att-1', url: '' });
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });
  it('rejects an already cancelled upload before initialization', async () => {
    const controller = new AbortController(); controller.abort();
    await expect(start({ signal: controller.signal })).rejects.toMatchObject({ name: 'AbortError' });
    expect(fetchMock).not.toHaveBeenCalled();
  });
  it('aborts initialization using the fetch signal', async () => {
    fetchMock.mockReset().mockImplementation((_url, { signal }) => new Promise((_resolve, reject) => signal.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError')))));
    const controller = new AbortController();
    const result = start({ signal: controller.signal });
    const rejected = expect(result).rejects.toMatchObject({ name: 'AbortError' });
    controller.abort();
    await rejected;
    expect(UploadXHR.current).toBeUndefined();
  });
  it('aborts the storage request without confirming and removes its abort listener', async () => {
    const controller = new AbortController();
    const remove = vi.spyOn(controller.signal, 'removeEventListener');
    const result = start({ signal: controller.signal });
    const rejected = expect(result).rejects.toMatchObject({ name: 'AbortError' });
    const xhr = await storage();
    controller.abort();
    await rejected;
    expect(xhr.abort).toHaveBeenCalledOnce();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(remove).toHaveBeenCalledWith('abort', expect.any(Function));
  });
  it('aborts confirmation instead of reporting completion', async () => {
    fetchMock.mockReset().mockResolvedValueOnce({ ok: true, json: async () => initialized }).mockImplementation((_url, { signal }) => new Promise((_resolve, reject) => signal.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError')))));
    const controller = new AbortController();
    const onProgress = vi.fn();
    const result = start({ signal: controller.signal, onProgress });
    const rejected = expect(result).rejects.toMatchObject({ name: 'AbortError' });
    (await storage()).onload?.();
    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    controller.abort();
    await rejected;
    expect(onProgress).not.toHaveBeenCalledWith(100);
  });
  it.each(['error', 'timeout', 'http'] as const)('reports storage %s without confirmation', async failure => {
    const result = start();
    const rejected = expect(result).rejects.toThrow(failure === 'timeout' ? /timed out/i : failure === 'http' ? /storage.*403/i : /connection/i);
    const xhr = await storage();
    if (failure === 'http') { xhr.status = 403; xhr.onload?.(); }
    else if (failure === 'timeout') xhr.ontimeout?.();
    else xhr.onerror?.();
    await rejected;
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
  it('aborts a storage request with no progress and offers a retry', async () => {
    vi.useFakeTimers();
    const result = start();
    const rejected = expect(result).rejects.toThrow(/stopped making progress.*retry/i);
    const xhr = await storage();
    await vi.advanceTimersByTimeAsync(30_000);
    await rejected;
    expect(xhr.abort).toHaveBeenCalledOnce();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(vi.getTimerCount()).toBe(0);
  });
  it('keeps an active transfer alive when bytes continue arriving', async () => {
    vi.useFakeTimers();
    const result = start();
    const xhr = await storage();
    await vi.advanceTimersByTimeAsync(20_000);
    xhr.upload.onprogress?.({ loaded: 2, total: 5, lengthComputable: true } as ProgressEvent);
    await vi.advanceTimersByTimeAsync(20_000);
    expect(xhr.abort).not.toHaveBeenCalled();
    xhr.onload?.();
    await expect(result).resolves.toEqual({ attachmentId: 'att-1', url: initialized.public_url });
    expect(vi.getTimerCount()).toBe(0);
  });
  it('does not extend the idle deadline for repeated zero-byte events', async () => {
    vi.useFakeTimers();
    const result = start();
    const rejected = expect(result).rejects.toThrow(/stopped making progress/i);
    const xhr = await storage();
    await vi.advanceTimersByTimeAsync(20_000);
    xhr.upload.onprogress?.({ loaded: 0, total: 5, lengthComputable: true } as ProgressEvent);
    await vi.advanceTimersByTimeAsync(10_000);
    await rejected;
    expect(xhr.abort).toHaveBeenCalledOnce();
    expect(vi.getTimerCount()).toBe(0);
  });
  it('preserves a helpful API validation error during initiation', async () => {
    fetchMock.mockReset().mockResolvedValue({ ok: false, status: 413, json: async () => ({ error: 'File exceeds the 100 MB limit' }) });
    await expect(start()).rejects.toThrow('File exceeds the 100 MB limit');
  });
  it('reports confirmation failures without completing progress', async () => {
    fetchMock.mockReset().mockResolvedValueOnce({ ok: true, json: async () => initialized }).mockResolvedValue({ ok: false, status: 500 });
    const onProgress = vi.fn();
    const result = start({ onProgress });
    const rejected = expect(result).rejects.toThrow(/confirm.*500/i);
    (await storage()).onload?.();
    await rejected;
    expect(onProgress).not.toHaveBeenCalledWith(100);
  });
  it('reports malformed initialization responses', async () => {
    fetchMock.mockReset().mockResolvedValue({ ok: true, json: async () => ({}) });
    await expect(start()).rejects.toThrow(/upload details/i);
  });
  it('times out initialization and releases the pending request', async () => {
    vi.useFakeTimers();
    fetchMock.mockReset().mockImplementation((_url, { signal }) => new Promise((_resolve, reject) => signal.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError')))));
    const result = start();
    const rejected = expect(result).rejects.toThrow(/timed out.*retry/i);
    await vi.advanceTimersByTimeAsync(10 * 60 * 1000);
    await rejected;
    expect(vi.getTimerCount()).toBe(0);
  });
});
