import type { SupportAttachmentPayload } from '@/lib/pmTypes';

export type PendingSupportAttachment = {
  localId: string;
  fileName: string;
  fileType: string;
  status: 'uploading' | 'done' | 'error';
  attachmentId?: string;
  previewUrl?: string;
  previewObjectUrl?: boolean;
};

export function restoreAttachmentsFromMessage(attachments: SupportAttachmentPayload[] = []): PendingSupportAttachment[] {
  return attachments.map((attachment) => ({
    localId: `restored-${attachment.id}`,
    fileName: attachment.file_name,
    fileType: attachment.file_type,
    status: 'done',
    attachmentId: attachment.id,
    previewUrl: attachment.file_type.startsWith('image/') ? attachment.url : undefined,
    previewObjectUrl: false,
  }));
}
