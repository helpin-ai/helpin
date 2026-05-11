import { pmAttachmentService } from '@/lib/services/pmAttachmentService';
import { uploadToS3 } from '@/lib/api';

export interface EditorUploadConfig {
  workspaceId: string;
  entityType: 'task' | 'epic' | 'objective' | 'sprint' | 'editor_upload';
  entityId: string;
}

export interface EditorImageUploadResult {
  attachmentId: string;
  publicUrl: string;
}

const MAX_SIZE = 50 * 1024 * 1024; // 50 MB

export async function uploadEditorFile(
  file: File,
  config: EditorUploadConfig,
): Promise<EditorImageUploadResult> {
  if (file.size > MAX_SIZE) {
    throw new Error('File exceeds maximum size of 50 MB');
  }

  // 1. Initiate upload → get presigned PUT URL + public URL
  const { data: initData, error: initError } = await pmAttachmentService.initiateUpload(
    config.workspaceId,
    {
      entity_type: config.entityType,
      entity_id: config.entityId,
      file_name: file.name || 'attachment',
      file_size: file.size,
      content_type: file.type || 'application/octet-stream',
    },
  );

  if (initError || !initData) {
    throw new Error(initError ?? 'Failed to initiate upload');
  }

  // 2. Upload directly to S3 with public-read ACL
  const { ok, error: s3Error } = await uploadToS3(
    initData.url,
    file,
    undefined,
    { 'x-amz-acl': 'public-read' },
  );
  if (!ok) {
    throw new Error(s3Error ?? 'Failed to upload to S3');
  }

  // 3. Confirm upload
  await pmAttachmentService.confirmUpload(config.workspaceId, initData.attachment.id);

  // 4. Return the permanent public URL
  if (!initData.public_url) {
    throw new Error('Server did not return a public URL');
  }
  return {
    attachmentId: initData.attachment.id,
    publicUrl: initData.public_url,
  };
}

/**
 * Upload an image file via the attachment infrastructure and return the public URL
 * together with the persistent attachment ID.
 */
export async function uploadEditorImage(
  file: File,
  config: EditorUploadConfig,
): Promise<EditorImageUploadResult> {
  if (!file.type.startsWith('image/')) {
    throw new Error('Only image files are supported');
  }
  return uploadEditorFile(file, config);
}
