import { useCallback, useEffect, useRef, useState } from 'preact/hooks';
import type { PendingAttachment } from '../types';

export interface AttachmentUploadOptions {
  signal?: AbortSignal;
  onProgress?: (percent: number) => void;
}
export type UploadAttachment = (file: File, localId: string, options?: AttachmentUploadOptions) => Promise<{ attachmentId: string; url: string } | null>;
export const MAX_SUPPORT_FILE_SIZE = 100 * 1024 * 1024;
export const SUPPORT_FILE_ACCEPT = 'image/*,video/mp4,video/quicktime,video/webm,video/mpeg,video/x-msvideo,video/x-matroska,.mp4,.mov,.webm,.mpeg,.mpg,.avi,.mkv,application/pdf,.doc,.docx,.txt,.csv,.xls,.xlsx,.zip,.gz,.tar,.md';
const videoTypes: Record<string, string> = { mp4: 'video/mp4', mov: 'video/quicktime', webm: 'video/webm', mpeg: 'video/mpeg', mpg: 'video/mpeg', avi: 'video/x-msvideo', mkv: 'video/x-matroska' };

export function useAttachmentUploads(scope: string, upload?: UploadAttachment) {
  const [pendingAttachments, setPending] = useState<PendingAttachment[]>([]);
  const [validationError, setValidationError] = useState('');
  const entries = useRef(new Map<string, { file: File; previewUrl?: string; controller?: AbortController }>());

  const clear = useCallback(() => {
    for (const entry of entries.current.values()) {
      entry.controller?.abort();
      if (entry.previewUrl) URL.revokeObjectURL(entry.previewUrl);
    }
    entries.current.clear();
    setPending([]);
    setValidationError('');
  }, []);
  const previousScope = useRef(scope);
  useEffect(() => {
    // The first message promotes the same mounted draft to a server ID.
    // Back/list navigation unmounts this hook; real conversation switches reset it.
    if (previousScope.current !== scope && previousScope.current !== '__new__') clear();
    previousScope.current = scope;
  }, [scope, clear]);
  useEffect(() => clear, [clear]);

  const run = useCallback(async (id: string) => {
    const entry = entries.current.get(id);
    if (!entry || !upload || entry.controller) return;
    const controller = new AbortController();
    entry.controller = controller;
    const active = () => entries.current.get(id) === entry && entry.controller === controller && !controller.signal.aborted;
    setPending(rows => rows.map(row => row.id === id ? { ...row, status: 'uploading', progress: 0, error: undefined } : row));
    try {
      const result = await upload(entry.file, id, {
        signal: controller.signal,
        onProgress: percent => {
          if (active() && Number.isFinite(percent)) setPending(rows => rows.map(row => row.id === id ? { ...row, progress: Math.min(99, Math.max(0, Math.round(percent))) } : row));
        },
      });
      if (!active()) return;
      if (!result) throw new Error('Upload failed. Please try again.');
      setPending(rows => rows.map(row => row.id === id ? { ...row, status: 'uploaded', progress: 100, attachmentId: result.attachmentId } : row));
    } catch (error) {
      if (active()) setPending(rows => rows.map(row => row.id === id ? { ...row, status: 'error', error: error instanceof Error ? error.message : 'Upload failed. Please try again.' } : row));
    } finally {
      if (entry.controller === controller) entry.controller = undefined;
    }
  }, [upload]);

  const select = useCallback(async (files: File[]) => {
    if (!upload) return;
    const rows: PendingAttachment[] = [];
    const errors: string[] = [];
    for (let file of files) {
      if (file.size > MAX_SUPPORT_FILE_SIZE) { errors.push(`${file.name} exceeds the 100 MB limit. Choose a smaller file or share a link.`); continue; }
      if (!file.size) { errors.push(`${file.name} is empty. Choose a different file.`); continue; }
      const extension = file.name.split('.').pop()?.toLowerCase() || '';
      const videoType = videoTypes[extension];
      if (videoType && (!file.type || file.type === 'application/octet-stream' || file.type === 'video/avi')) file = new File([file], file.name, { type: videoType, lastModified: file.lastModified });
      const id = `pending-${Date.now()}-${Math.random().toString(36).slice(2)}`;
      const previewUrl = file.type.startsWith('image/') && file.type !== 'image/heic' && file.type !== 'image/heif' ? URL.createObjectURL(file) : undefined;
      entries.current.set(id, { file, previewUrl });
      rows.push({ id, fileName: file.name, fileType: file.type || 'application/octet-stream', fileSize: file.size, previewUrl, status: 'uploading', progress: 0 });
    }
    setValidationError(errors.join(' '));
    // Register the entire batch before starting so Send cannot drop queued files.
    setPending(previous => [...previous, ...rows]);
    for (const row of rows) await run(row.id);
  }, [upload, run]);

  const remove = useCallback((id: string) => {
    const entry = entries.current.get(id);
    entries.current.delete(id);
    entry?.controller?.abort();
    if (entry?.previewUrl) URL.revokeObjectURL(entry.previewUrl);
    setPending(rows => rows.filter(row => row.id !== id));
  }, []);
  return { pendingAttachments, validationError, select, remove, retry: run, clear };
}
