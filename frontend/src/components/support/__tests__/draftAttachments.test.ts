import { describe, expect, it } from 'vitest';
import { restoreAttachmentsFromMessage } from '../draftAttachments';
import type { SupportAttachmentPayload } from '@/lib/pmTypes';

function attachment(overrides: Partial<SupportAttachmentPayload>): SupportAttachmentPayload {
  return {
    id: 'att-1',
    file_key: 'support/att-1',
    file_name: 'screenshot.png',
    file_type: 'image/png',
    file_size: 1234,
    url: 'https://cdn.example.com/screenshot.png',
    ...overrides,
  };
}

describe('restoreAttachmentsFromMessage', () => {
  it('restores uploaded message attachments as done composer attachments', () => {
    expect(restoreAttachmentsFromMessage([
      attachment({ id: 'image-1', file_name: 'image.png', file_type: 'image/png', url: 'https://cdn.example.com/image.png' }),
      attachment({ id: 'file-1', file_name: 'terms.pdf', file_type: 'application/pdf', url: 'https://cdn.example.com/terms.pdf' }),
    ])).toEqual([
      {
        localId: 'restored-image-1',
        fileName: 'image.png',
        fileType: 'image/png',
        status: 'done',
        attachmentId: 'image-1',
        previewUrl: 'https://cdn.example.com/image.png',
        previewObjectUrl: false,
      },
      {
        localId: 'restored-file-1',
        fileName: 'terms.pdf',
        fileType: 'application/pdf',
        status: 'done',
        attachmentId: 'file-1',
        previewUrl: undefined,
        previewObjectUrl: false,
      },
    ]);
  });
});
