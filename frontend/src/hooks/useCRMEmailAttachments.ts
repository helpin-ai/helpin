import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { toast } from 'sonner';
import { uploadToS3 } from '@/lib/api';
import { unwrap } from '@/lib/queryUtils';
import { crmEmailService } from '@/lib/services/crmService';

const MAX_FILES = 10;
const MAX_FILE_SIZE = 10 * 1024 * 1024;
const MAX_TOTAL_SIZE = 16 * 1024 * 1024;

export interface PendingCRMEmailAttachment {
  id: string;
  fileName: string;
  fileSize: number;
  status: 'uploading' | 'done' | 'error';
  progress: number;
}

function newDraftId() {
  if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID();
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (character) => {
    const value = Math.floor(Math.random() * 16);
    return (character === 'x' ? value : (value & 0x3) | 0x8).toString(16);
  });
}

export interface CRMEmailAttachmentDraft { draftId: string; attachments: PendingCRMEmailAttachment[] }

export function useCRMEmailAttachments(workspaceId: string, initialDraft?: CRMEmailAttachmentDraft, onDraftChange?: (draft: CRMEmailAttachmentDraft) => void) {
  const [draftId, setDraftId] = useState(() => initialDraft?.draftId ?? newDraftId());
  const [attachments, setAttachments] = useState<PendingCRMEmailAttachment[]>(initialDraft?.attachments ?? []);
  const [pendingUploads, setPendingUploads] = useState(0);
  const uploadLock = useRef(false);
  useEffect(() => { onDraftChange?.({ draftId, attachments }); }, [draftId, attachments, onDraftChange]);
  const totalSize = useMemo(() => attachments.reduce((sum, item) => sum + item.fileSize, 0), [attachments]);

  const uploadFiles = useCallback(async (files: File[]) => {
    if (attachments.length + files.length > MAX_FILES) { toast.error('You can attach up to 10 files'); return; }
    if (files.some((file) => file.size > MAX_FILE_SIZE)) { toast.error('Each attachment must be 10MB or smaller'); return; }
    if (totalSize + files.reduce((sum, file) => sum + file.size, 0) > MAX_TOTAL_SIZE) { toast.error('Attachments must be 16MB or smaller in total'); return; }
    if (uploadLock.current) return;
    uploadLock.current = true;
    setPendingUploads(files.length);
    for (const file of files) {
      let attachmentId = '';
      try {
        const initiated = unwrap(await crmEmailService.createAttachment(workspaceId, {
          draft_id: draftId,
          file_name: file.name,
          file_size: file.size,
          content_type: file.type || 'application/octet-stream',
        }));
        attachmentId = initiated.attachment.id;
        setAttachments((current) => [...current, { id: attachmentId, fileName: file.name, fileSize: file.size, status: 'uploading', progress: 0 }]);
        const uploaded = await uploadToS3(initiated.upload_url, file, (progress) => setAttachments((current) => current.map((item) => item.id === attachmentId ? { ...item, progress } : item)));
        if (!uploaded.ok) throw new Error(uploaded.error || 'Upload failed');
        await unwrap(await crmEmailService.confirmAttachment(workspaceId, attachmentId));
        setAttachments((current) => current.map((item) => item.id === attachmentId ? { ...item, status: 'done', progress: 100 } : item));
      } catch (error) {
        if (attachmentId) setAttachments((current) => current.map((item) => item.id === attachmentId ? { ...item, status: 'error' } : item));
        toast.error(error instanceof Error ? error.message : `Could not attach ${file.name}`);
      }
    }
    uploadLock.current = false;
    setPendingUploads(0);
  }, [attachments.length, draftId, totalSize, workspaceId]);

  const pickFiles = useCallback(() => {
    const input = document.createElement('input');
    input.type = 'file'; input.multiple = true;
    input.onchange = () => { if (input.files?.length) void uploadFiles(Array.from(input.files)); };
    input.click();
  }, [uploadFiles]);

  const remove = useCallback(async (id: string) => {
    setAttachments((current) => current.filter((item) => item.id !== id));
    try { await unwrap(await crmEmailService.deleteAttachment(workspaceId, id)); } catch { /* stale drafts are cleaned server-side */ }
  }, [workspaceId]);

  const reset = useCallback(() => { setDraftId(newDraftId()); setAttachments([]); }, []);
  return {
    draftId,
    attachments,
    hasFailedUploads: attachments.some((item) => item.status === 'error'),
    attachmentIds: attachments.filter((item) => item.status === 'done').map((item) => item.id),
    uploading: pendingUploads > 0 || attachments.some((item) => item.status === 'uploading'),
    pickFiles,
    remove,
    reset,
  };
}
