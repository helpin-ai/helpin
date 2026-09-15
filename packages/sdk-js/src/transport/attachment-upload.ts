import { widgetURL } from '../core/urls';
export interface AttachmentUploadOptions {
  signal?: AbortSignal;
  onProgress?: (percent: number) => void;
}

// Covers initialization, the direct storage upload, and server confirmation.
const UPLOAD_TIMEOUT_MS = 10 * 60 * 1000;
const timeoutMessage = 'The file upload timed out. Check your connection and retry.';
const aborted = () => new DOMException('File upload cancelled.', 'AbortError');

function checkAborted(signal: AbortSignal): void {
  if (signal.aborted) throw aborted();
}

async function responseError(response: Response, fallback: string): Promise<Error> {
  try {
    const body = await response.json();
    const message = typeof body.error === 'string' ? body.error : body.message;
    if (typeof message === 'string' && message.trim()) return new Error(message);
  } catch {
    // Storage and proxy errors may not have a JSON body.
  }
  return new Error(`${fallback} (HTTP ${response.status}). Please retry.`);
}

function uploadToStorage(url: string, file: File, publicRead: boolean, signal: AbortSignal, onProgress?: (percent: number) => void): Promise<void> {
  return new Promise((resolve, reject) => {
    checkAborted(signal);
    const xhr = new XMLHttpRequest();
    const cleanup = () => {
      signal.removeEventListener('abort', cancel);
      xhr.onload = xhr.onerror = xhr.onabort = xhr.ontimeout = null;
      xhr.upload.onprogress = null;
    };
    const fail = (error: Error) => { cleanup(); reject(error); };
    const cancel = () => { xhr.abort(); fail(aborted()); };
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) { cleanup(); resolve(); }
      else fail(new Error(`Unable to upload the file to storage (HTTP ${xhr.status}). Please retry.`));
    };
    xhr.onerror = () => fail(new Error('Unable to upload the file. Check your connection and retry.'));
    xhr.onabort = () => fail(aborted());
    xhr.ontimeout = () => fail(new Error(timeoutMessage));
    xhr.upload.onprogress = event => {
      if (event.lengthComputable && event.total > 0) {
        // 100% means confirmed and ready to send, not just transferred to storage.
        onProgress?.(Math.min(99, Math.max(0, Math.round(event.loaded / event.total * 100))));
      }
    };
    signal.addEventListener('abort', cancel, { once: true });
    try {
      xhr.open('PUT', url, true);
      xhr.timeout = UPLOAD_TIMEOUT_MS;
      xhr.setRequestHeader('Content-Type', file.type || 'application/octet-stream');
      if (publicRead) xhr.setRequestHeader('x-amz-acl', 'public-read');
      xhr.send(file);
    } catch (error) {
      fail(error instanceof Error ? error : new Error('Unable to start the file upload. Please retry.'));
    }
  });
}

export async function uploadAttachment(
  host: string,
  sessionToken: string,
  file: File,
  options: AttachmentUploadOptions = {},
): Promise<{ attachmentId: string; url: string }> {
  if (options.signal?.aborted) throw aborted();
  if (!sessionToken) throw new Error('Chat is not connected yet. Please wait and retry the upload.');
  const controller = new AbortController();
  const { signal } = controller;
  const cancel = () => controller.abort();
  options.signal?.addEventListener('abort', cancel, { once: true });
  let timedOut = false;
  const timeout = setTimeout(() => { timedOut = true; controller.abort(); }, UPLOAD_TIMEOUT_MS);
  let stage = 'start the file upload';
  try {
    options.onProgress?.(0);
    checkAborted(signal);
    const headers = { 'Content-Type': 'application/json', 'X-Session-Token': sessionToken };
    const init = await fetch(widgetURL(host, '/widget/support/attachments'), {
      method: 'POST', headers, signal,
      body: JSON.stringify({ file_name: file.name, file_size: file.size, content_type: file.type || 'application/octet-stream' }),
    });
    checkAborted(signal);
    if (!init.ok) throw await responseError(init, 'Unable to start the file upload');
    const data = await init.json();
    checkAborted(signal);
    const attachmentId = data.attachment?.id;
    if (typeof attachmentId !== 'string' || !attachmentId || typeof data.upload_url !== 'string' || !data.upload_url) {
      throw new Error('The server did not return valid upload details. Please retry.');
    }
    stage = 'upload the file to storage';
    const publicUrl = typeof data.public_url === 'string' ? data.public_url : '';
    await uploadToStorage(data.upload_url, file, !!publicUrl, signal, options.onProgress);
    checkAborted(signal);
    stage = 'confirm the file upload';
    const confirm = await fetch(widgetURL(host, `/widget/support/attachments/${attachmentId}/confirm`), {
      method: 'PATCH', headers, signal,
    });
    checkAborted(signal);
    if (!confirm.ok) throw await responseError(confirm, 'Unable to confirm the file upload');
    checkAborted(signal);
    options.onProgress?.(100);
    return { attachmentId, url: publicUrl };
  } catch (error) {
    if (timedOut) throw new Error(timeoutMessage);
    if (signal.aborted) throw aborted();
    if (error instanceof TypeError) throw new Error(`Unable to ${stage}. Check your connection and retry.`);
    throw error;
  } finally {
    clearTimeout(timeout);
    options.signal?.removeEventListener('abort', cancel);
  }
}
